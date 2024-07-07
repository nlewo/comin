overlay: { config, pkgs, lib, ... }:
let
  cfg = config;
  yaml = pkgs.formats.yaml { };
  cominConfig = {
    hostname = cfg.services.comin.hostname;
    state_dir = "/var/lib/comin";
    flake_subdirectory = cfg.services.comin.flake_subdirectory;
    remotes = cfg.services.comin.remotes;
    exporter = {
      listen_address = cfg.services.comin.exporter.listen_address;
      port = cfg.services.comin.exporter.port;
    };
  };
  cominConfigYaml = yaml.generate "comin.yaml" cominConfig;
in {
  imports = [ ./module-options.nix ];
  config = lib.mkIf cfg.services.comin.enable {
    nixpkgs.overlays = [ overlay ];
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
}
