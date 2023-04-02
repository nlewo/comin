{
  description = "Comin";

  outputs = { self, nixpkgs }:
  let
    system = "x86_64-linux";
    pkgs = import nixpkgs {
      system = "x86_64-linux";
      overlays = [ self.overlay ];
    };
  in {
    overlay = final: prev: {
      comin = pkgs.buildGoModule rec {
        pname = "comin";
        version = "0.0.1";
        # TODO: fix tests in sandbox :/
        doCheck = false;
        src = pkgs.lib.cleanSourceWith {
          src = ./.;
          filter = path: type:
          let
            p = baseNameOf path;
          in !(
            p == "flake.nix" ||
            p == "flake.lock" ||
            p == "README.md"
          );
        };
        vendorSha256 = "sha256-zhTRR0NQ67Hf4NyqZ+j8R7wHiVMQcCTBFsi344lJLUA=";
      };
    };

    packages.x86_64-linux.comin = pkgs.comin;
    defaultPackage.x86_64-linux = pkgs.comin;

    nixosModules.comin = { config, pkgs, lib, ... }: let
      cfg = config;
      yaml = pkgs.formats.yaml { };
      cominConfig = {
        hostname = config.networking.hostName;
        state_dir = "/var/lib/comin";
        remotes = [
          {
            name = "origin";
            url = cfg.services.comin.repository;
            auth = {
              access_token_path = cfg.services.comin.authFile;
            };
          }
        ];
        branches = {
          main = {
            name = "main";
            protected = true;
          };
          testing = {
            name = cfg.services.comin.testingBranch;
            protected = false;
          };
        };
        poller.period = cfg.services.comin.pollerPeriod;
      };
      cominConfigYaml = yaml.generate "comin.yml" cominConfig;
    in {
      options = {
        services.comin = {
          enable = lib.mkOption {
            type = lib.types.bool;
            default = false;
            description = ''
              Whether to run the comin service.
            '';
          };
          repository = lib.mkOption {
            type = lib.types.str;
            description = ''
              The repository to poll.
            '';
          };
          authFile = lib.mkOption {
            type = lib.types.nullOr lib.types.str;
            default = null;
            description = ''
              The path of the auth file.
            '';
          };
          testingBranch = lib.mkOption {
            type = lib.types.str;
            default = "testing-${cfg.networking.hostName}";
            description = ''
              The name of the testing branch.
            '';
          };
          pollerPeriod = lib.mkOption {
            type = lib.types.int;
            default = 60;
            description = ''
              The poller period in seconds.
            '';
          };
          debug = lib.mkOption {
            type = lib.types.bool;
            default = false;
            description = ''
              Whether to run comin in debug mode. Be careful, secrets are shown!.
            '';
          };
          machineId = lib.mkOption {
            type = lib.types.nullOr lib.types.str;
            default = null;
            description = ''
              The expected machine-id of the machine configured by
              comin. If not null, the configuration is only deployed
              when this specified machine-id is equal to the actual
              machine-id.
              This is mainly useful for server migration: this allows
              to migrate a configuration from a machine to another
              machine (with different hardware for instance) without
              impacting both.
              Note it is only used by comin at evaluation.
            '';
          };
        };
      };
      config = lib.mkIf cfg.services.comin.enable {
        nixpkgs.overlays = [ self.overlay ];
        systemd.services.comin = {
          wantedBy = [ "multi-user.target" ];
          path = [ pkgs.nix pkgs.git ];
          # The comin service is restart by comin itself when it
          # detects the unit file changed.
          restartIfChanged = false;
          serviceConfig = {
            ExecStart =
              "${pkgs.comin}/bin/comin "
              + (lib.optionalString cfg.services.comin.debug "--debug ")
              + " poll "
              + "--config ${cominConfigYaml}";
              Restart = "always";
          };
        };
      };
    };

    devShell.x86_64-linux = let
      pkgs = nixpkgs.legacyPackages.x86_64-linux;
    in pkgs.mkShell {
      buildInputs = [
        pkgs.go pkgs.godef
      ];
    };
  };
}
