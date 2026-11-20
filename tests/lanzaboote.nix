{
  lib,
  self,
  nixpkgsFor,
  lanzaboote,
  lanzabootePackage,
  cominPackage,
  ...
}:

let
  sortKey = "mySpecialSortKey";

  pkgs = nixpkgsFor.x86_64-linux;

  # We need to explicitly add lanzaboote inputs to avoid having to download them in the test
  inputs = map (i: i.outPath) (builtins.attrValues self.inputs ++ (builtins.attrValues self.inputs.lanzaboote.inputs));
  gitRepository = pkgs.runCommand "git-repository" { } ''
    mkdir $out
    cd $out
    echo ${../.}
    cp -r ${../.}/* ./
    cp -r ${../.}/.* ./
  '';

  
  # Generate test keys at build time for signing the UKIs
  keys = pkgs.runCommand "lanzaboote-test-keys" {
    nativeBuildInputs = [ pkgs.sbctl ];
  } ''
    mkdir -p $out
    cat > sbctl.conf <<EOF
    keydir: $out/keys
    guid: $out/GUID
    EOF
    sbctl create-keys --config sbctl.conf
  '';

  imageModule =
    {
      config,
      lib,
      pkgs,
      modulesPath,
      ...
    }:
    let
      espLabel = "esp";

      cfg = config.boot.lanzaboote;

      genlist = pkgs.writeText "genlist.json" (
        builtins.toJSON {
          version = 1;
          outpath = config.system.build.toplevel;
          build_time = 1;
          label = "commit-id:42424242";
        }
      );

      espFiles = pkgs.runCommand "esp-files" { } ''
        mkdir -p $out
        ${cfg.installCommand} \
          --public-key ${cfg.publicKeyFile} \
          --private-key ${cfg.privateKeyFile} \
          $out \
          --genlist-path ${genlist}
      '';

      closureInfo = pkgs.closureInfo {
        rootPaths = [ config.system.build.toplevel lanzabootePackage ];
      };

      nixState = pkgs.runCommand "nix-state" { nativeBuildInputs = [ pkgs.nix ]; } ''
        mkdir -p $out/profiles
        ln -s ${config.system.build.toplevel} $out/profiles/system-1-link
        ln -s /nix/var/nix/profiles/system-1-link $out/profiles/system

        export NIX_STATE_DIR=$out
        nix-store --load-db < ${closureInfo}/registration
      '';
    in
    {
      imports = [ "${modulesPath}/image/repart.nix" ];

      fileSystems = {
        "/boot" = {
          device = "/dev/disk/by-partlabel/${espLabel}";
          fsType = "vfat";
          options = [ "umask=077" ];
        };
        "/" = {
          fsType = "tmpfs";
          options = [ "mode=755" ];
        };
        "/nix/store" = {
          overlay = {
            lowerdir = [ "/nix/.ro-store" ];
            upperdir = "/nix/.rw-store/upper";
            workdir = "/nix/.rw-store/work";
          };
        };
        "/nix/.rw-store" = {
          fsType = "tmpfs";
          options = [ "mode=0755" ];
          neededForBoot = true;
        };
      };

      system.image = {
        id = config.system.name;
        version = config.system.nixos.version;
      };

      system.build.espFiles = espFiles;
      system.extraDependencies = inputs ++ [ gitRepository ];

      boot.initrd.availableKernelModules = [
        "virtiofs"
      ];

      image.repart = {
        enable = true;
        name = config.system.name;
        mkfsOptions = {
          # Makes the build significantly faster by slimming down the images.
          erofs = [ "-z lz4" ];
        };
        partitions = {
          "esp" = {
            contents = {
              "/".source = espFiles;
            };
            repartConfig = {
              Type = "esp";
              Format = config.fileSystems."/boot".fsType;
              Label = espLabel;
              SizeMinBytes = if config.nixpkgs.hostPlatform.isx86_64 then "64M" else "128M";
              UUID = "a3c9c5a1-1a9a-451c-bdac-a80bacb4170b";
            };
          };
          "root" = {
            contents = {
              "/nix/var/nix".source = nixState;
            };
            repartConfig = {
              Type = "root";
              Format = "ext4";
              Label = "root";
              SizeMinBytes = "1G";
            };
          };
        };
      };

      virtualisation = {
        directBoot.enable = false;
        mountHostNixStore = true;
        writableStore = true;
        memorySize = 2048;
        fileSystems = lib.mkForce {
          "/" = {
            device = "/dev/disk/by-partlabel/root";
            fsType = "ext4";
          };
          "/boot" = {
            device = "/dev/disk/by-partlabel/${espLabel}";
            fsType = "vfat";
            options = [ "umask=077" ];
          };
          "/nix/store" = {
            overlay = {
              lowerdir = [ "/nix/.ro-store" ];
              upperdir = "/nix/.rw-store/upper";
              workdir = "/nix/.rw-store/work";
            };
          };
          "/nix/.ro-store" = {
            fsType = "virtiofs";
            device = "nix-store";
            neededForBoot = true;
            options = [ "ro" ];
          };
          "/nix/.rw-store" = {
            fsType = "tmpfs";
            options = [ "mode=0755" ];
            neededForBoot = true;
          };
        };
        useDefaultFilesystems = false;

        useEFIBoot = true;
      };
    };

  lanzabooteModule = {
    config,
    lib,
    pkgs,
    ...
  }: {
    imports = [ imageModule ];

    config = {
      system.extraDependencies = inputs ++ [ gitRepository ];
      boot = {
        loader.timeout = 0;
        loader.efi.canTouchEfiVariables = true;

        lanzaboote = {
          enable = true;
          logLevel = "debug";
          publicKeyFile = "${keys}/keys/db/db.pem";
          privateKeyFile = "${keys}/keys/db/db.key";
        };
      };
    };
  };

  imageHelper =
    { machine, ... }:
    ''
      import os
      import subprocess
      import tempfile

      tmp_disk_image = tempfile.NamedTemporaryFile()

      subprocess.run([
        "${machine.virtualisation.qemu.package}/bin/qemu-img",
        "create",
        "-f",
        "qcow2",
        "-b",
        "${machine.system.build.image}/${machine.image.fileName}",
        "-F",
        "raw",
        tmp_disk_image.name,
      ])

      # Set NIX_DISK_IMAGE so that the qemu script finds the right disk image.
      os.environ['NIX_DISK_IMAGE'] = tmp_disk_image.name

      # Enroll keys via systemd-boot by rebooting
      ${builtins.replaceStrings ["-"] ["_"] machine.networking.hostName}.start(allow_reboot=True)
      ${builtins.replaceStrings ["-"] ["_"] machine.networking.hostName}.connected = False
    '';

in
pkgs.testers.runNixOSTest {
  name = "lanzaboote";

  interactive.sshBackdoor.enable = true;
  nodes.lanzaboote-test = {
    imports = [
      lanzaboote
      lanzabooteModule
      (import ../nix/module.nix { inherit self; })
    ];

    boot.lanzaboote = {
      inherit sortKey;
      package = lanzabootePackage;
      genlistPath = "/nix/var/nix/profiles/genlist.json";
    };
    system.switch.enable = true;
    services.comin = {
      enable = true;
      package = cominPackage;
      remotes = [
        {
        name = "local";
        url = "/root/git-repository";
        branches.main.operation = "boot";
        }
      ];
    };
  };

  testScript =
    { nodes, ... }:
    (imageHelper { machine = nodes.lanzaboote-test; })
    + ''
      bootctl_status = lanzaboote_test.succeed("bootctl status")
      print(bootctl_status)
      t.assertIn("sort-key: ${sortKey}", bootctl_status)

      lanzaboote_test.succeed("mkdir /root/git-repository")
      lanzaboote_test.succeed("cp -r ${gitRepository}/* /root/git-repository/")
      lanzaboote_test.succeed("cp -r ${gitRepository}/.* /root/git-repository/")
      lanzaboote_test.succeed("${pkgs.git}/bin/git -C /root/git-repository/ init -b main")
      lanzaboote_test.succeed("${pkgs.git}/bin/git -C /root/git-repository/ config user.name 'Alice'")
      lanzaboote_test.succeed("${pkgs.git}/bin/git -C /root/git-repository/ config user.email 'alice@bob.com'")
      lanzaboote_test.succeed("${pkgs.git}/bin/git -C /root/git-repository/ add -A")
      lanzaboote_test.succeed("${pkgs.git}/bin/git -C /root/git-repository/ commit -m init")

      # status, output = lanzaboote_test.execute("nix --extra-experimental-features 'nix-command flakes' derivation show /root/git-repository/tests#nixosConfigurations.lanzaboote_test.config.system.build.toplevel")
      # print(output)
      lanzaboote_test.succeed("comin fetch")
      lanzaboote_test.wait_until_succeeds("comin status --json | ${pkgs.jq}/bin/jq -e '.deployer.deployment.error_msg == \"\"'")


      # We want systemd to recognize our PE binaries as true UKIs. systemd has
      # become more picky in the past, so make sure.
      t.assertIn("Kernel Type: uki", lanzaboote_test.succeed("bootctl kernel-inspect /boot/EFI/Linux/nixos-generation-1-*.efi"))

      # Verify comin service is present
      lanzaboote_test.succeed("systemctl status comin.service")
    '';
}
