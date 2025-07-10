{ config, pkgs, lib, ... }:
let
  cfg = config;
  yaml = pkgs.formats.yaml { };
in rec {
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
  } // (
    lib.optionalAttrs (cfg.services.comin.postDeploymentCommand != null)
      { post_deployment_command = cfg.services.comin.postDeploymentCommand; }
  );
  cominConfigYaml = yaml.generate "comin.yaml" cominConfig;
}
