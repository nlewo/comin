# Tooling for making docs & not expected to be used downstream
# Future improvement: making each script callPackage-able.
final: _prev:

let
  inherit (final) lib;
in
{
  comin-tools = lib.makeScope final.newScope (self: {
    optionsDocCommonMark =
      let
        eval = lib.evalModules {
          modules = [
            (import ../module-options.nix)
            {
              options = {
                _module.args = lib.mkOption {
                  internal = true;
                };
                assertions = lib.mkOption {
                  type = lib.types.anything;
                  description = "";
                };

                networking.hostName = lib.mkOption {
                  type = lib.types.str;
                  internal = true;
                  default = "the-machine-hostname";
                };
              };
            }
          ];
        };
        optionsDoc = final.nixosOptionsDoc {
          inherit (eval) options;
        };
      in
      final.runCommand "options-doc.md" { } ''
        awk 'NF { last = NR } { lines[NR] = $0 } END { for (i = 1; i <= last; i++) print lines[i] }' ${optionsDoc.optionsCommonMark} > $out
      '';

    optionsDocCommonMarkGenerator = final.writers.writeBashBin "optionsDocCommonMarkGenerator" ''
      cp -v ${self.optionsDocCommonMark} ./docs/generated-module-options.md
      chmod u+w ./docs/generated-module-options.md
    '';

    checkOptionsDocCommonMark =
      let
        generated = ../../docs/generated-module-options.md;
      in
      final.runCommand "check-options-doc.md" { } ''
        set +e
        ${lib.getBin final.diffutils}/bin/diff -q ${self.optionsDocCommonMark} ${generated}
        if [[ $? -ne 0 ]]
        then
          echo "The ./docs/generated-module-options.md file is not up to date."
          echo "Run 'nix run .#generate-module-options' to generate it!"
          exit 1
        fi
        echo Files ${self.optionsDocCommonMark} ${generated} are identical > $out
      '';
  });
}
