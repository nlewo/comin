{ self }:
{
  config,
  pkgs,
  lib,
  ...
}:
let
  cfg = config;
  cominConfigLib = import ./comin-config.nix { inherit config pkgs lib; };
  inherit (cominConfigLib) cominConfigYaml;

  inherit (pkgs.stdenv.hostPlatform) system;
  inherit (cfg.services.comin) package;

  remoteWithAuth = lib.findFirst (r: r.auth.access_token_path != "") null cfg.services.comin.remotes;

  # This is needed because Nix's flake fetcher shells out to git for
  # submodule operations, and git has no other way to authenticate.
  gitAskpass = pkgs.writeShellScript "comin-git-askpass" ''
    case "$1" in
      Username*) echo "${remoteWithAuth.auth.username}" ;;
      Password*) cat "${remoteWithAuth.auth.access_token_path}" ;;
    esac
  '';
in
{
  imports = [ ./module-options.nix ];
  config = lib.mkIf cfg.services.comin.enable {
    assertions = [
      {
        assertion = package != null;
        message = "`services.comin.package` cannot be null.";
      }
      # If the package is null and our `system` isn't supported by the Flake, it's probably safe to show this error message
      {
        assertion = package == null -> lib.elem system (lib.attrNames self.packages);
        message = "comin: ${system} is not supported by the Flake.";
      }
    ];

    systemd.user.services.comin-desktop = lib.mkIf cfg.services.comin.desktop.enable {
      wantedBy = [ "graphical-session.target" ];
      path = [ pkgs.libnotify ];
      serviceConfig = {
        ExecStart = ''${lib.getExe package} desktop --title "${cfg.services.comin.desktop.title}"'';
      };
    };

    environment.systemPackages = [ package ];
    networking.firewall.allowedTCPPorts = lib.optional cfg.services.comin.exporter.openFirewall cfg.services.comin.exporter.port;
    # Use package from overlay first, then Flake package if available
    services.comin.package = lib.mkDefault pkgs.comin or self.packages.${system}.comin or null;
    systemd.services.comin = {
      wantedBy = [ "multi-user.target" ];
      path = [
        config.nix.package
        config.programs.ssh.package
      ];
      # The comin service is restarted by comin itself when it
      # detects the unit file changed.
      restartIfChanged = false;
      environment = lib.mkIf (cfg.services.comin.submodules && remoteWithAuth != null) {
        GIT_ASKPASS = gitAskpass;
      };
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
