{ comin }:
comin.overrideAttrs (old: {
  name = "go-test";
  doCheck = true;
  installPhase = ''
    touch $out
  '';
  fixupPhase = ":";
})
