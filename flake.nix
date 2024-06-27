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
    overlays.default = final: prev: let
      # - safe.directory: this is to allow comin to fetch local repositories belonging
      #   to other users. Otherwise, comin fails with:
      #   Pull from remote 'local' failed: unknown error: fatal: detected dubious ownership in repository
      # - core.hooksPath: to avoid Git executing hooks from a repository belonging to another user
      gitConfigFile = final.writeTextFile {
        name = "git.config";
        text = ''
          [safe]
             directory = *
          [core]
             hooksPath = /dev/null
        '';
      };
    in {
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
        vendorHash = "sha256-Gt1FQWHp8yfSxS6IztRaQN07HjQg7qR9OOTA5oXpXdk=";
        ldflags = [
          "-X github.com/nlewo/comin/cmd.version=${version}"
        ];
        buildInputs = [ final.makeWrapper ];
        postInstall = ''
          # This is because Nix needs Git at runtime by the go-git library
          wrapProgram $out/bin/comin --set GIT_CONFIG_SYSTEM ${gitConfigFile} --prefix PATH : ${final.git}/bin
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
