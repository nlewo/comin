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
      path = [ config.nix.package pkgs.systemd ];
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

    
    # We use an external "restarter" service instead of relying on systemd's automatic
    # restart of the comin service because:
    # 
    # - `services.comin.restartIfChanged = false` prevents NixOS from restarting comin
    #   when its unit file or dependencies change during a switch.
    # - The comin service self-updates and calls `switch-to-configuration`, then exits
    #   to let systemd restart it, but that restart happens using the *previous*
    #   in-memory unit definition — not the newly generated one.
    # - A full `systemctl restart comin.service` is required after `daemon-reload` to
    #   rebind systemd to the new unit file from the current generation.

    systemd.services.comin-restarter = {
      serviceConfig = {
        Type = "oneshot";
        ExecStart = ''
          ${pkgs.systemd}/bin/systemctl restart comin.service
        '';
      };
    };
  };
}
