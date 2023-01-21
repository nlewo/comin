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
        vendorSha256 = "sha256-PQ8UpiIpOs2NQ6SYD1aIIvZ3wNkIrFNoUkylpO0aV8c=";
      };
    };

    packages.x86_64-linux.comin = pkgs.comin;
    defaultPackage.x86_64-linux = pkgs.comin;

    nixosModules.comin = { config, pkgs, lib, ... }: let
      cfg = config.services.comin;
      yaml = pkgs.formats.yaml { };
      cominConfig = {
        hostname = config.networking.hostName;
        state_dir = "/var/lib/comin";
        remotes = [
          {
            name = "origin";
            url = cfg.repository;
            auth = {
              access_token_path = cfg.authFile;
            };
          }
        ];
        branches = {
          main = {
            name = "main";
            protected = true;
          };
          testing = {
            name = "testing";
            protected = false;
          };
        };
        poller.period = 10;
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
          debug = lib.mkOption {
            type = lib.types.bool;
            default = false;
            description = ''
              Whether to run comin in debug mode. Be careful, secrets are shown!.
            '';
          };
        };
      };
      config = {
        nixpkgs.overlays = [ self.overlay ];
        systemd.services.comin = {
          wantedBy = [ "multi-user.target" ];
          path = [ pkgs.nix pkgs.git ];
          # FIXME find a better way to restart comin without breaking
          # the current deployment executed by comin...
          restartIfChanged = false;
          serviceConfig = {
            ExecStart =
              "${pkgs.comin}/bin/comin "
              + (lib.optionalString cfg.debug "--debug ")
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
