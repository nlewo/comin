self: { config, pkgs, lib, ... }:
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
  };
  cominConfigYaml = yaml.generate "comin.yaml" cominConfig;

  inherit (pkgs.stdenv.hostPlatform) system;
  package = self.packages.${system}.comin;
in {
  imports = [ ./module-options.nix ];
  config = lib.mkIf cfg.services.comin.enable {
    assertions = [ { assertion = lib.elem system (lib.attrNames self.packages); message = "comin: ${system} is not supported by the Flake"; } ];
    environment.systemPackages = [ package ];
    networking.firewall.allowedTCPPorts = lib.optional cfg.services.comin.exporter.openFirewall cfg.services.comin.exporter.port;
    systemd.services.comin = {
      wantedBy = [ "multi-user.target" ];
      path = [ config.nix.package ];
      # The comin service is restarted by comin itself when it
      # detects the unit file changed.
      restartIfChanged = false;
      serviceConfig = {
        ExecStart =
          "${package}/bin/comin "
          + (lib.optionalString cfg.services.comin.debug "--debug ")
          + " run "
          + "--config ${cominConfigYaml}";
        Restart = "always";
      };
    };
  };
}
