{
  lib,
  buildGoModule,
  git,
  makeWrapper,
  writeTextFile,
}:

let
  # - safe.directory: this is to allow comin to fetch local repositories belonging
  #   to other users. Otherwise, comin fails with:
  #   Pull from remote 'local' failed: unknown error: fatal: detected dubious ownership in repository
  # - core.hooksPath: to avoid Git executing hooks from a repository belonging to another user
  gitConfigFile = writeTextFile {
    name = "git.config";
    text = ''
      [safe]
         directory = *
      [core]
         hooksPath = /dev/null
    '';
  };
in

buildGoModule rec {
  pname = "comin";
  version = "0.11.0";
  nativeCheckInputs = [ git ];
  # We run tests in the go-test derivation to speedup the comin build
  doCheck = false;
  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../cmd
      ../internal
      ../go.mod
      ../go.sum
      ../main.go
    ];
  };
  vendorHash = "sha256-niH4c9+aVfVYSyfviCbVVA1xuYu1BWmfWz317VTDcqg=";
  ldflags = [
    "-X github.com/nlewo/comin/cmd.version=${version}"
  ];
  nativeBuildInputs = [ makeWrapper ];
  postInstall = ''
    # This is because Nix needs Git at runtime by the go-git library
    wrapProgram $out/bin/comin --set GIT_CONFIG_SYSTEM ${gitConfigFile} --prefix PATH : ${lib.makeBinPath [ git ]}
  '';

  meta = {
    mainProgram = "comin";
  };
}
