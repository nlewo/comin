# golangci-lint needs a vendor directory
{ pkgs, comin }:
comin.overrideAttrs (old: {
  name = "golangci-lint";
  nativeBuildInputs = old.nativeBuildInputs ++ [ pkgs.golangci-lint ];
  buildPhase = ''
    HOME=$TMPDIR golangci-lint run --timeout 360s
  '';
  doCheck = false;
  installPhase = ''
    touch $out $unittest
  '';
  fixupPhase = ":";
})
