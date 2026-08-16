{
  config,
  pkgs,
  lib,
  ...
}:
let
  cfg = config;
  yaml = pkgs.formats.yaml { };
in
rec {
  cominConfig = {
    hostname = cfg.services.comin.hostname;
    state_dir = "/var/lib/comin";

    fetcher.type = cfg.services.comin.fetcher.type;
    fetcher.git.repository_type = cfg.services.comin.fetcher.git.repositoryType;
    fetcher.git.repository_subdir = cfg.services.comin.fetcher.git.repositorySubdir;
    fetcher.git.submodules = cfg.services.comin.fetcher.git.submodules;
    fetcher.git.system_attr = cfg.services.comin.fetcher.git.systemAttr;
    fetcher.git.remotes = cfg.services.comin.fetcher.git.remotes;

    fetcher.nixk3.remotes = cfg.services.comin.fetcher.niks3.remotes;

    exporter = {
      listen_address = cfg.services.comin.exporter.listen_address;
      port = cfg.services.comin.exporter.port;
    };
    gpg_public_key_paths = cfg.services.comin.gpgPublicKeyPaths;
    build_confirmer = cfg.services.comin.buildConfirmer;
    deploy_confirmer = cfg.services.comin.deployConfirmer;
    retention = cfg.services.comin.retention;
    eval_timeout = cfg.services.comin.evalTimeout;
    build_timeout = cfg.services.comin.buildTimeout;
  }
  // (lib.optionalAttrs (cfg.services.comin.sshAllowedSignersPath != null) {
    ssh_allowed_signers_path = cfg.services.comin.sshAllowedSignersPath;
  })
  // (lib.optionalAttrs (cfg.services.comin.postDeploymentCommand != null) {
    post_deployment_command = cfg.services.comin.postDeploymentCommand;
  });
  cominConfigYaml = yaml.generate "comin.yaml" cominConfig;
}
