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
      { assertion = package == null -> lib.elem system (lib.attrNames self.packages); message = "comin: ${system} is not supported by the Flake."; }
      { assertion = cfg.services.comin.hostname != null && cfg.services.comin.hostname != ""; message = "You must set `networking.hostName` or `services.comin.hostname` explicitly in your nix-darwin configuration. Comin requires an explicit hostname to determine which darwinConfiguration to deploy."; }
    ];

    environment.systemPackages = [ package ];
    services.comin.package = lib.mkDefault pkgs.comin or self.packages.${system}.comin or null;
    launchd.daemons.comin = {
      serviceConfig = {
        ProgramArguments = [
          (lib.getExe package)
        ] ++ (lib.optionals cfg.services.comin.debug [ "--debug" ]) ++ [
          "run"
          "--config"
          "${cominConfigYaml}"
        ];
        Label = "com.github.nlewo.comin";
        KeepAlive = true;
        RunAtLoad = true;
        StandardErrorPath = "/var/log/comin.log";
        StandardOutPath = "/var/log/comin.log";
        EnvironmentVariables = {
          PATH = lib.makeBinPath [ config.nix.package pkgs.git ];
        };
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
  };
}