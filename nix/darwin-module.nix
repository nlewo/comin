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
  inherit (cominConfigLib) cominConfig cominConfigYaml;

  inherit (pkgs.stdenv.hostPlatform) system;
  inherit (cfg.services.comin) package;
in
{
  imports = [ ./module-options.nix ];
  config = lib.mkIf cfg.services.comin.enable {
    assertions = [
      {
        assertion = package != null;
        message = "`services.comin.package` cannot be null.";
      }
      {
        assertion = package == null -> lib.elem system (lib.attrNames self.packages);
        message = "comin: ${system} is not supported by the Flake.";
      }
    ];

    environment.systemPackages = [ package ];
    services.comin.package = lib.mkDefault pkgs.comin or self.packages.${system}.comin or null;

    # The plist only names /etc paths, so that nix-darwin does not reload comin during the activations it runs.
    environment.etc = {
      "comin/comin".source = lib.getExe package;
      "comin/config.yaml".source = cominConfigYaml;
      "comin/path".source = pkgs.buildEnv {
        name = "comin-path";
        paths = [
          config.nix.package
          pkgs.git
          pkgs.openssh
        ];
      };
    };
    launchd.daemons.comin = {
      command = lib.concatStringsSep " " (
        [ "/etc/comin/comin" ]
        ++ (lib.optionals cfg.services.comin.debug [ "--debug" ])
        ++ [
          "run"
          "--config"
          "/etc/comin/config.yaml"
        ]
      );
      serviceConfig = {
        Label = "com.github.nlewo.comin";
        KeepAlive = true;
        RunAtLoad = true;
        StandardErrorPath = "/var/log/comin.log";
        StandardOutPath = "/var/log/comin.log";
        EnvironmentVariables.PATH = "/etc/comin/path/bin";
      };
    };

    system.activationScripts.comin.text = ''
      mkdir -p /var/lib/comin
      chown root:wheel /var/lib/comin
      chmod 755 /var/lib/comin
    '';

    # Override launchd reload behavior for comin service to prevent hanging
    # Comin manages its own restart through the deployment process
    system.activationScripts.extraActivation.text = lib.mkAfter ''
      # Skip automatic reload of comin service - it manages its own lifecycle
      if [ -f /Library/LaunchDaemons/com.github.nlewo.comin.plist ]; then
        # Ensure service is loaded but don't restart during activation
        /bin/launchctl load -w /Library/LaunchDaemons/com.github.nlewo.comin.plist 2>/dev/null || true

        # Check if the service is actually running, if not start it
        if ! /bin/launchctl list | grep -q "com.github.nlewo.comin"; then
          echo "Comin service not running, starting it..."
          /bin/launchctl start com.github.nlewo.comin || true
        fi
      fi
    '';

    # comin restarts itself after its own deployments once they are recorded
    system.activationScripts.postActivation.text = lib.mkAfter ''
      cominInputs="${package} ${cominConfigYaml}"
      if [ "$(cat /var/lib/comin/inputs 2>/dev/null)" != "$cominInputs" ]; then
        mkdir -p /var/lib/comin
        echo "$cominInputs" > /var/lib/comin/inputs
        if [ -z "''${COMIN_ACTIVATION:-}" ]; then
          /bin/launchctl kickstart -k system/com.github.nlewo.comin || true
        fi
      fi
    '';
  };
}
