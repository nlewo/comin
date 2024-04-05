pkgs: rec {
  optionsDocCommonMark = let
    eval = pkgs.lib.evalModules {
      modules = [
        (import ./module-options.nix)
        {
          options = {
            _module.args = pkgs.lib.mkOption {
              internal = true;
            };
            networking.hostName = pkgs.lib.mkOption {
              type = pkgs.lib.types.str;
              internal = true;
              default = "the-machine-hostname";
            };
          };
        }
      ];
    };
    optionsDoc = pkgs.nixosOptionsDoc {
      inherit (eval) options;
    };
  in
    pkgs.runCommand "options-doc.md" {} ''
      cat ${optionsDoc.optionsCommonMark} >> $out
    '';
  optionsDocCommonMarkGenerator = pkgs.writers.writeBashBin "optionsDocCommonMarkGenerator" ''
    cp -v ${optionsDocCommonMark} ./docs/generated-module-options.md
    chmod u+w ./docs/generated-module-options.md
  '';
  checkOptionsDocCommonMark =  pkgs.runCommand "check-options-doc.md" {} ''
    set +e
    ${pkgs.diffutils}/bin/diff -q ${optionsDocCommonMark} ${../docs/generated-module-options.md}
    if [[ $? -ne 0 ]]
    then
      echo "The ./docs/module-options.md file is not up to date."
      echo "Run 'nix run .#generate-module-options' to generate it!"
      exit 1
    fi
    echo Files ${optionsDocCommonMark} ${../docs/generated-module-options.md} are identical > $out
  '';
}
