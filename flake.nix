{
  description = "Comin - Git Push NixOS Machines";

  outputs = { self, nixpkgs }:
  let
    systems = [ "aarch64-linux" "x86_64-linux" ];
    forAllSystems = nixpkgs.lib.genAttrs systems;
    nixpkgsFor = forAllSystems (system: import nixpkgs {
      inherit system;
      overlays = [ self.overlay ];
    });
  in {
    overlay = final: prev: {
      comin = final.buildGoModule rec {
        pname = "comin";
        version = "0.1.1";
        nativeCheckInputs = [ final.git ];
        src = final.lib.cleanSourceWith {
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
        vendorHash = "sha256-9qObgfXvMkwE+1BVZNQXVhKhL6LqMqyIUhGnXf8q9SI=";
        buildInputs = [ final.makeWrapper ];
        postInstall = ''
          # This is because Nix needs Git at runtime by the go-git library
          wrapProgram $out/bin/comin --prefix PATH : ${final.git}/bin
        '';
      };
    };

    packages = forAllSystems (system: { inherit (nixpkgsFor."${system}") comin; });
    defaultPackage = forAllSystems (system: self.packages."${system}".comin);

    nixosModules.comin = { config, pkgs, lib, ... }: let
      cfg = config;
      yaml = pkgs.formats.yaml { };
      cominConfig = {
        hostname = cfg.services.comin.hostname;
        state_dir = "/var/lib/comin";
        remotes = cfg.services.comin.remotes;
        exporter = {
          listen_address = cfg.services.comin.exporter.listen_address;
          port = cfg.services.comin.exporter.port;
        };
      };
      cominConfigYaml = yaml.generate "comin.yaml" cominConfig;
    in {
      options = with lib; with types; {
        services.comin = {
          enable = mkOption {
            type = types.bool;
            default = false;
            description = ''
              Whether to run the comin service.
            '';
          };
          hostname = mkOption {
            type = str;
            default = config.networking.hostName;
            description = ''
              The name of the NixOS configuration to evaluate and
              deploy. This value is used by comin to evaluate the
              flake output
              nixosConfigurations."<hostname>".config.system.build.toplevel
            '';
          };
          exporter = mkOption {
            description = "Options for the Prometheus exporter.";
            default = {};
            type = submodule {
              options = {
                listen_address = mkOption {
                  type = str;
                  description = ''
                    Address to listen on for the Prometheus exporter. Empty string will listen on all interfaces.
                  '';
                  default = "";
                };
                port = mkOption {
                  type = int;
                  description = ''
                    Port to listen on for the Prometheus exporter.
                  '';
                  default = 4243;
                };
                openFirewall = mkOption {
                  type = types.bool;
                  default = false;
                  description = lib.mdDoc ''
                    Open port in firewall for incoming connections to the Prometheus exporter.
                  '';
                };
              };
            };
          };
          remotes = mkOption {
            description = "Ordered list of repositories to pull";
            type = listOf (submodule {
              options = {
                name = mkOption {
                  type = str;
                  description = ''
                    The name of the remote.
                  '';
                };
                url = mkOption {
                  type = str;
                  description = ''
                    The URL of the repository.
                  '';
                };
                auth = mkOption {
                  description = "Authentication options";
                  default = {};
                  type = submodule {
                    options = {
                      access_token_path = mkOption {
                        type = str;
                        default = "";
                        description = ''
                          The path of the auth file.
                        '';
                      };
                    };
                  };
                };
                timeout = mkOption {
                  type = int;
                  default = 300;
                  description = ''
                    Git fetch timeout in seconds.
                  '';
                };
                branches = mkOption {
                  description = "Branches to pull";
                  default = {};
                  type = submodule {
                    options = {
                      main = mkOption {
                        default = {};
                        type = submodule {
                          options = {
                            name = mkOption {
                              type = str;
                              default = "main";
                              description = "The name of the main branch.";
                            };
                          };
                        };
                      };
                      testing = mkOption {
                        default = {};
                        type = submodule {
                          options = {
                            name = mkOption {
                              type = str;
                              default = "testing-${cfg.services.comin.hostname}";
                              description = "The name of the testing branch.";
                            };
                          };
                        };
                      };
                    };
                  };
                };
                poller = mkOption {
                  default = {};
                  type = submodule {
                    options = {
                      period = mkOption {
                        type = types.int;
                        default = 60;
                        description = ''
                          The poller period in seconds.
                        '';
                      };
                    };
                  };
                };
              };
            });
          };
          debug = mkOption {
            type = types.bool;
            default = false;
            description = ''
              Whether to run comin in debug mode. Be careful, secrets are shown!.
            '';
          };
          machineId = mkOption {
            type = types.nullOr types.str;
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
        environment.systemPackages = [ pkgs.comin ];
        networking.firewall.allowedTCPPorts = lib.optional cfg.services.comin.exporter.openFirewall cfg.services.comin.exporter.port;
        systemd.services.comin = {
          wantedBy = [ "multi-user.target" ];
          path = [ config.nix.package ];
          # The comin service is restarted by comin itself when it
          # detects the unit file changed.
          restartIfChanged = false;
          serviceConfig = {
            ExecStart =
              "${pkgs.comin}/bin/comin "
              + (lib.optionalString cfg.services.comin.debug "--debug ")
              + " run "
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
        pkgs.go pkgs.godef pkgs.gopls
      ];
    };
  };
}
