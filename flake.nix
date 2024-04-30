{
  description = "Comin - GitOps for NixOS Machines";

  outputs = { self, nixpkgs }:
  let
    systems = [ "aarch64-linux" "x86_64-linux" ];
    forAllSystems = nixpkgs.lib.genAttrs systems;
    nixpkgsFor = forAllSystems (system: import nixpkgs {
      inherit system;
      overlays = [ self.overlays.default ];
    });
    optionsDocFor = forAllSystems (system:
      import ./nix/module-options-doc.nix (nixpkgsFor."${system}")
    );
  in {
    overlays.default = final: prev: {
      comin = final.buildGoModule rec {
        pname = "comin";
        version = "0.2.0";
        nativeCheckInputs = [ final.git ];
        src = final.lib.fileset.toSource {
          root = ./.;
          fileset = final.lib.fileset.unions [
            ./cmd
            ./internal
            ./go.mod
            ./go.sum
            ./main.go
          ];
        };
        vendorHash = "sha256-9qObgfXvMkwE+1BVZNQXVhKhL6LqMqyIUhGnXf8q9SI=";
        ldflags = [
          "-X github.com/nlewo/comin/cmd.version=${version}"
        ];
        buildInputs = [ final.makeWrapper ];
        postInstall = ''
          # This is because Nix needs Git at runtime by the go-git library
          wrapProgram $out/bin/comin --prefix PATH : ${final.git}/bin
        '';
      };
    };

    packages = forAllSystems (system: {
      default = nixpkgsFor."${system}".comin;
      generate-module-options = optionsDocFor."${system}".optionsDocCommonMarkGenerator;
    });
    checks = forAllSystems (system: {
      module-options-doc = optionsDocFor."${system}".checkOptionsDocCommonMark;
      # I don't understand why nix flake check does't build packages.default
      package = nixpkgsFor."${system}".comin;
    });

    nixosModules.comin = import ./nix/module.nix self.overlays.default;
    devShells.x86_64-linux.default = let
      pkgs = nixpkgs.legacyPackages.x86_64-linux;
    in pkgs.mkShell {
      buildInputs = [
        pkgs.go pkgs.godef pkgs.gopls
      ];
    };
  };
}
