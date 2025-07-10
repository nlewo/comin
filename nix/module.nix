{ self }: { config, pkgs, lib, ... }:
let
  cfg = config;
  cominConfigLib = import ./comin-config.nix { inherit config pkgs lib; };
  inherit (cominConfigLib) cominConfig cominConfigYaml;

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
      path = [ config.nix.package ];
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
