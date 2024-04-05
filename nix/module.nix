comin: { config, pkgs, lib, ... }: let
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
  imports = [ ./module-options.nix ];
  config = lib.mkIf cfg.services.comin.enable {
    environment.systemPackages = [ comin ];
    networking.firewall.allowedTCPPorts = lib.optional cfg.services.comin.exporter.openFirewall cfg.services.comin.exporter.port;
    systemd.services.comin = {
      wantedBy = [ "multi-user.target" ];
      path = [ pkgs.nix pkgs.git ];
      # The comin service is restarted by comin itself when it
      # detects the unit file changed.
      restartIfChanged = false;
      serviceConfig = {
        ExecStart =
          "${comin}/bin/comin "
          + (lib.optionalString cfg.services.comin.debug "--debug ")
          + " run "
          + "--config ${cominConfigYaml}";
          Restart = "always";
      };
    };
  };
}
