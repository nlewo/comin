{
  description = "Comin - Git Push NixOS Machines";

  outputs = { self, nixpkgs }:
  let
    systems = [ "aarch64-linux" "x86_64-linux" ];
    forAllSystems = nixpkgs.lib.genAttrs systems;
    nixpkgsFor = forAllSystems (system: import nixpkgs {
      inherit system;
      overlays = [ self.overlay ];
    });
    optionsDocFor = forAllSystems (system:
      import ./nix/module-options-doc.nix (nixpkgsFor."${system}")
    );
  in {
    overlay = final: prev: {
      comin = final.buildGoModule rec {
        pname = "comin";
        version = "0.2.0";
        nativeCheckInputs = [ final.git ];
        src = final.lib.cleanSourceWith {
          src = ./.;
          filter = path: type:
          let
            p = baseNameOf path;
          in !(
            p == "flake.nix" ||
            p == "flake.lock" ||
            p == "README.md"
          );
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
      inherit (nixpkgsFor."${system}") comin;
      generate-module-options = optionsDocFor."${system}".optionsDocCommonMarkGenerator;
    });
    defaultPackage = forAllSystems (system: self.packages."${system}".comin);
    checks = forAllSystems (system: {
      options-doc = optionsDocFor."${system}".checkOptionsDocCommonMark;
    });

    nixosModules.comin = import ./nix/module.nix self.overlay;
    devShell.x86_64-linux = let
      pkgs = nixpkgs.legacyPackages.x86_64-linux;
    in pkgs.mkShell {
      buildInputs = [
        pkgs.go pkgs.godef pkgs.gopls
      ];
    };
  };
}
