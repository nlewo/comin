{ self }:
{
  config,
  pkgs,
  lib,
  ...
}:
let
  cfg = config.services.comin;
  yaml = pkgs.formats.yaml { };

  cominConfig = {
    hostname = cfg.hostname;
    state_dir = "/var/lib/comin";
    repository_type = "system-manager";
    repository_subdir = cfg.repositorySubdir;
    submodules = cfg.submodules;
    remotes = cfg.remotes;
    exporter = {
      listen_address = cfg.exporter.listen_address;
      port = cfg.exporter.port;
    };
    gpg_public_key_paths = cfg.gpgPublicKeyPaths;
    build_confirmer = cfg.buildConfirmer;
    deploy_confirmer = cfg.deployConfirmer;
    retention = cfg.retention;
  }
  // (lib.optionalAttrs (cfg.postDeploymentCommand != null) {
    post_deployment_command = cfg.postDeploymentCommand;
  });

  cominConfigYaml = yaml.generate "comin.yaml" cominConfig;

  inherit (pkgs.stdenv.hostPlatform) system;

  remoteWithAuth = lib.findFirst (r: r.auth.access_token_path != "") null cfg.remotes;

  gitAskpass = pkgs.writeShellScript "comin-git-askpass" ''
    case "$1" in
      Username*) echo "${remoteWithAuth.auth.username}" ;;
      Password*) cat "${remoteWithAuth.auth.access_token_path}" ;;
    esac
  '';
in
{
  options.services.comin =
    with lib;
    with types;
    {
      enable = mkOption {
        type = bool;
        default = false;
        description = ''
          Whether to run the comin service for system-manager deployments.
        '';
      };
      package = mkOption {
        type = nullOr package;
        default = self.packages.${system}.comin or null;
        defaultText = literalExpression "self.packages.\${system}.comin or null";
        description = ''
          The comin package to use.
        '';
      };
      hostname = mkOption {
        type = str;
        description = ''
          The name of the configuration to evaluate and deploy.
          This value is used by comin to evaluate the flake output
          systemConfigs."<hostname>".
          Unlike the NixOS module, this must be set explicitly since
          system-manager does not have networking.hostName.
        '';
      };
      repositorySubdir = mkOption {
        type = str;
        default = ".";
        description = ''
          Subdirectory in the repository containing a flake.nix file.
        '';
      };
      submodules = mkOption {
        type = bool;
        default = false;
        description = ''
          Whether to fetch and include Git submodules when cloning the repository.
        '';
      };
      exporter = mkOption {
        description = "Options for the Prometheus exporter.";
        default = { };
        type = submodule {
          options = {
            listen_address = mkOption {
              type = str;
              description = "Address to listen on for the Prometheus exporter.";
              default = "";
            };
            port = mkOption {
              type = int;
              description = "Port to listen on for the Prometheus exporter.";
              default = 4243;
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
              description = "The name of the remote.";
            };
            url = mkOption {
              type = str;
              description = "The URL of the repository.";
            };
            auth = mkOption {
              description = "Authentication options.";
              default = { };
              type = submodule {
                options = {
                  access_token_path = mkOption {
                    type = str;
                    default = "";
                    description = "The path of the auth file.";
                  };
                  username = mkOption {
                    type = str;
                    default = "comin";
                    description = "The username used to authenticate to the Git remote repository.";
                  };
                };
              };
            };
            timeout = mkOption {
              type = int;
              default = 300;
              description = "Git fetch timeout in seconds.";
            };
            branches = mkOption {
              description = "Branches to pull.";
              default = { };
              type = submodule {
                options = {
                  main = mkOption {
                    default = { };
                    description = "The main branch to fetch.";
                    type = submodule {
                      options = {
                        name = mkOption {
                          type = str;
                          default = "main";
                          description = "The name of the main branch.";
                        };
                        operation = mkOption {
                          type = enum [
                            "switch"
                            "test"
                          ];
                          default = "switch";
                          description = "The operation to do on this branch. 'switch' registers and activates. 'test' only activates without registering a new profile.";
                        };
                      };
                    };
                  };
                  testing = mkOption {
                    default = { };
                    description = "The testing branch to fetch.";
                    type = submodule {
                      options = {
                        name = mkOption {
                          type = str;
                          default = "testing-${cfg.hostname}";
                          defaultText = lib.literalExpression "testing-\${config.services.comin.hostname}";
                          description = "The name of the testing branch.";
                        };
                        operation = mkOption {
                          type = enum [
                            "switch"
                            "test"
                          ];
                          default = "test";
                          description = "The operation to do on this branch. 'switch' registers and activates. 'test' only activates without registering a new profile.";
                        };
                      };
                    };
                  };
                };
              };
            };
            poller = mkOption {
              default = { };
              description = "The poller options.";
              type = submodule {
                options = {
                  period = mkOption {
                    type = int;
                    default = 60;
                    description = "The poller period in seconds.";
                  };
                };
              };
            };
          };
        });
      };
      debug = mkOption {
        type = bool;
        default = false;
        description = "Whether to run comin in debug mode. Be careful, secrets are shown!";
      };
      machineId = mkOption {
        type = nullOr str;
        default = null;
        description = ''
          The expected machine-id of the machine configured by comin.
          If not null, the configuration is only deployed when this
          specified machine-id is equal to the actual machine-id.
        '';
      };
      gpgPublicKeyPaths = mkOption {
        description = "A list of GPG public key file paths.";
        type = listOf str;
        default = [ ];
      };
      postDeploymentCommand = mkOption {
        description = "A path to a script executed after each deployment.";
        type = nullOr path;
        default = null;
      };
      buildConfirmer = mkOption {
        description = "The confirmer options for the build.";
        default = { };
        type = submodule {
          options = {
            mode = mkOption {
              type = enum [
                "without"
                "auto"
                "manual"
              ];
              default = "without";
              description = "The confirmer mode.";
            };
            autoconfirm_duration = mkOption {
              type = int;
              default = 120;
              description = "The autoconfirm timer duration in seconds.";
            };
          };
        };
      };
      deployConfirmer = mkOption {
        description = "The confirmer options for the deployment.";
        default = { };
        type = submodule {
          options = {
            mode = mkOption {
              type = enum [
                "without"
                "auto"
                "manual"
              ];
              default = "without";
              description = "The confirmer mode.";
            };
            autoconfirm_duration = mkOption {
              type = int;
              default = 120;
              description = "The autoconfirm timer duration in seconds.";
            };
          };
        };
      };
      retention = mkOption {
        description = "The deployments and profiles retention policies.";
        default = { };
        type = submodule {
          options = {
            deployment_boot_entry_capacity = mkOption {
              type = int;
              default = 3;
              description = "Number of boot entries to keep.";
            };
            deployment_successful_capacity = mkOption {
              type = int;
              default = 3;
              description = "Number of successful deployments to keep.";
            };
            deployment_any_capacity = mkOption {
              type = int;
              default = 5;
              description = "Total number of deployments to keep.";
            };
          };
        };
      };
    };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = cfg.package != null;
        message = "`services.comin.package` cannot be null.";
      }
      {
        assertion = cfg.hostname != null && cfg.hostname != "";
        message = "You must set `services.comin.hostname` explicitly for system-manager deployments.";
      }
    ];

    environment.systemPackages = [ cfg.package ];

    systemd.services.comin = {
      wantedBy = [ "system-manager.target" ];
      path = [ pkgs.nix pkgs.openssh pkgs.git ];
      # comin restarts itself when it detects its unit file changed
      restartIfChanged = false;
      environment = lib.mkIf (cfg.submodules && remoteWithAuth != null) {
        GIT_ASKPASS = gitAskpass;
      };
      serviceConfig = {
        ExecStart =
          (lib.getExe cfg.package)
          + (lib.optionalString cfg.debug " --debug ")
          + " run "
          + "--config ${cominConfigYaml}";
        Restart = "always";
      };
    };
  };
}
