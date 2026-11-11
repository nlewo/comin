{
  lib,
  self,
  nixpkgsFor,
  cominPackage,
  ...
}:

let
  pkgs = nixpkgsFor.x86_64-linux;
  # To fill the Nix store with all eval inputs
  inputs = map (i: i.outPath) (builtins.attrValues self.inputs);
  gitRepository = pkgs.runCommand "git-repository" { } ''
    mkdir $out
    cd $out
    echo ${../.}
    cp -r ${../.}/* ./
    cp -r ${../.}/.* ./
  '';

in
pkgs.testers.runNixOSTest {
  name = "comin";
  interactive.sshBackdoor.enable = true;
  nodes.machine = {
    imports = [
      (import ../nix/module.nix { inherit self; })
    ];
    system.extraDependencies = inputs ++ [ gitRepository ];
    environment.systemPackages = [ pkgs.nix ];
    services.comin = {
      enable = true;
      package = cominPackage;
      remotes = [
        {
        name = "local";
        url = "/root/git-repository";
        }
      ];
      repositorySubdir = "tests";
    };
  };

  testScript = ''
    machine.start()
    machine.wait_for_unit("multi-user.target")
    
    # Verify comin service is present and running
    #    machine.succeed("systemctl is-active comin.service")
    machine.succeed("mkdir /root/git-repository")
    machine.succeed("cp -r ${gitRepository}/* /root/git-repository/")
    machine.succeed("cp -r ${gitRepository}/.* /root/git-repository/")
    machine.succeed("${pkgs.git}/bin/git -C /root/git-repository/ init -b main")
    machine.succeed("${pkgs.git}/bin/git -C /root/git-repository/ config user.name 'Alice'")
    machine.succeed("${pkgs.git}/bin/git -C /root/git-repository/ config user.email 'alice@bob.com'")
    machine.succeed("${pkgs.git}/bin/git -C /root/git-repository/ add -A")
    machine.succeed("${pkgs.git}/bin/git -C /root/git-repository/ commit -m init")

    # status, output = machine.execute("nix --extra-experimental-features 'nix-command flakes' derivation show /root/git-repository/tests#nixosConfigurations.machine.config.system.build.toplevel")
    # print(output)
    machine.succeed("comin fetch")
    machine.wait_until_succeeds("comin status --json | ${pkgs.jq}/bin/jq -e '.deployer.deployment.error_msg == \"\"'")
  '';
}
