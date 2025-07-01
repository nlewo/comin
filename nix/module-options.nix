{ config, pkgs, lib, ... }: {
  options = with lib; with types; {
    services.comin = {
      enable = mkOption {
        type = types.bool;
        default = false;
        description = ''
          Whether to run the comin service.
        '';
      };
      package = lib.mkPackageOption pkgs "comin" { nullable = true; } // {
        defaultText = "pkgs.comin or comin.packages.\${system}.default or null";
      };
      hostname = mkOption {
        type = str;
        default = config.networking.hostName;
        description = ''
          The name of the configuration to evaluate and deploy. 
          This value is used by comin to evaluate the flake output
          nixosConfigurations."<hostname>" or darwinConfigurations."<hostname>".
          Defaults to networking.hostName - you MUST set either this option
          or networking.hostName in your configuration.
        '';
      };
      flakeSubdirectory = mkOption {
        type = str;
        default = ".";
        description = ''
          Subdirectory in the repository, containing flake.nix.
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
              description = ''
                Open port in firewall for incoming connections to the Prometheus exporter.
              '';
            };
          };
        };
      };
      remotes = mkOption {
        description = "Ordered list of repositories to pull.";
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
              description = "Authentication options.";
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
              description = "Branches to pull.";
              default = {};
              type = submodule {
                options = {
                  main = mkOption {
                    default = {};
                    description = "The main branch to fetch.";
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
                    description = "The testing branch to fetch.";
                    type = submodule {
                      options = {
                        name = mkOption {
                          type = str;
                          default = "testing-${config.services.comin.hostname}";
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
              description = "The poller options.";
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
      gpgPublicKeyPaths = mkOption {
        description = "A list of GPG public key file paths. Each of this file should contains an armored GPG key.";
        type = listOf str;
        default = [];
      };
      postDeploymentCommand = mkOption {
        description = "A path to a script executed after each
        deployment. comin provides to the script the following
        environment variables: `COMIN_GIT_SHA`, `COMIN_GIT_REF`,
        `COMIN_GIT_MSG`, `COMIN_HOSTNAME`, `COMIN_FLAKE_URL`,
        `COMIN_GENERATION`, `COMIN_STATUS` and `COMIN_ERROR_MSG`.";
        type = nullOr path;
        default = null;
        example = lib.literalExpression ''
          pkgs.writers.writeBash "post" "echo $COMIN_GIT_SHA";
        '';
      };
    };
  };
}
