{
  description = "Comin - GitOps for NixOS Machines";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs";
  inputs.treefmt-nix.url = "github:numtide/treefmt-nix";
  inputs.flake-compat = {
    url = "github:NixOS/flake-compat";
    flake = false;
  };

  outputs =
    {
      self,
      nixpkgs,
      treefmt-nix,
      flake-compat,
    }:
    let
      systems = [
        "aarch64-linux"
        "x86_64-linux"
        "aarch64-darwin"
        "x86_64-darwin"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
      nixpkgsFor = forAllSystems (system: nixpkgs.legacyPackages.${system});
      optionsDocFor = forAllSystems (
        system: import ./nix/module-options-doc.nix (nixpkgsFor."${system}")
      );
      treefmtStack = forAllSystems (
        system:
        treefmt-nix.lib.evalModule nixpkgsFor.${system} {
          projectRootFile = "flake.nix";
          programs.nixfmt.enable = true;
          programs.nixfmt.package = nixpkgsFor.${system}.nixfmt;
        }
      );
    in
    {
      overlays.default = final: prev: {
        comin = final.callPackage ./nix/package.nix { };
      };

      formatter = forAllSystems (system: treefmtStack.${system}.config.build.wrapper);

      packages = forAllSystems (system: {
        comin = nixpkgsFor."${system}".callPackage ./nix/package.nix { };
        default = self.packages."${system}".comin;
        generate-module-options = optionsDocFor."${system}".optionsDocCommonMarkGenerator;
      });
      checks = forAllSystems (system: {
        module-options-doc = optionsDocFor."${system}".checkOptionsDocCommonMark;
        # I don't understand why nix flake check does't build packages.default
        package = self.packages."${system}".comin;
        formatting = treefmtStack.${system}.config.build.check self;
      });

      nixosModules.comin = nixpkgs.lib.modules.importApply ./nix/module.nix { inherit self; };
      darwinModules.comin = nixpkgs.lib.modules.importApply ./nix/darwin-module.nix { inherit self; };
      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          default = pkgs.mkShell {
            nativeBuildInputs = [ treefmtStack.${system}.config.build.wrapper ];
            buildInputs = with pkgs; [
              godef
              gopls
              golangci-lint
              protobuf
              protoc-gen-go
              protoc-gen-go-grpc
              buf
            ];
          };
        }
      );
    };
}
