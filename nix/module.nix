{ self }: { config, pkgs, lib, ... }:
let
  cfg = config;
  yaml = pkgs.formats.yaml { };
  cominConfig = {
    hostname = cfg.services.comin.hostname;
    state_dir = "/var/lib/comin";
    flake_subdirectory = cfg.services.comin.flakeSubdirectory;
    remotes = cfg.services.comin.remotes;
    exporter = {
      listen_address = cfg.services.comin.exporter.listen_address;
      port = cfg.services.comin.exporter.port;
    };
    gpg_public_key_paths = cfg.services.comin.gpgPublicKeyPaths;
    allow_force_push_main = cfg.services.comin.allowForcePushMain;
  };
  cominConfigYaml = yaml.generate "comin.yaml" cominConfig;

  inherit (pkgs.stdenv.hostPlatform) system;
  inherit (cfg.services.comin) package;
in {
  imports = [ ./module-options.nix ];
  config = lib.mkIf cfg.services.comin.enable {
    assertions = [
      { assertion = package != null; message = "`services.comin.package` cannot be null."; }
      # If the package is null and our `system` isn't supported by the Flake, it's probably safe to show this error message
      { assertion = package == null -> lib.elem system (lib.attrNames self.packages); message = "comin: ${system} is not supported by the Flake."; }
      { assertion = cfg.services.comin.hostname != null && cfg.services.comin.hostname != ""; message = "You must set `networking.hostName` or `services.comin.hostname` explicitly in your NixOS configuration. Comin requires an explicit hostname to determine which nixosConfiguration to deploy."; }
    ];

    environment.systemPackages = [ package ];
    networking.firewall.allowedTCPPorts = lib.optional cfg.services.comin.exporter.openFirewall cfg.services.comin.exporter.port;
    # Use package from overlay first, then Flake package if available
    services.comin.package = lib.mkDefault pkgs.comin or self.packages.${system}.comin or null;
    systemd.services.comin = {
      wantedBy = [ "multi-user.target" ];
      path = [ config.nix.package config.programs.ssh.package ];
      # The comin service is restarted by comin itself when it
      # detects the unit file changed.
      restartIfChanged = false;
      serviceConfig = {
        ExecStart =
          (lib.getExe package)
          + (lib.optionalString cfg.services.comin.debug " --debug ")
          + " run "
          + "--config ${cominConfigYaml}";
        Restart = "always";
      };
    };
  };
}
