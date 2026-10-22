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
      nixpkgsFor = forAllSystems (
        system:
        import nixpkgs {
          inherit system;
          overlays = [
            self.overlays.default
            self.overlays.tools
          ];
        }
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
      overlays = {
        default = import nix/overlay;
        tools = import nix/overlay/tools.nix;
      };

      formatter = forAllSystems (system: treefmtStack.${system}.config.build.wrapper);

      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          inherit (pkgs) comin;
          default = self.packages."${system}".comin;
          generate-module-options = pkgs.comin-tools.optionsDocCommonMarkGenerator;
        }
      );
      checks = forAllSystems (
        system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          module-options-doc = pkgs.comin-tools.checkOptionsDocCommonMark;
          # I don't understand why nix flake check does't build packages.default
          package = self.packages."${system}".comin;
          formatting = treefmtStack.${system}.config.build.check self;
          lint = pkgs.callPackage ./nix/lint.nix { };
        }
      );
      nixosModules.comin = ./nix/module.nix;
      darwinModules.comin = ./nix/darwin-module.nix;
      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          default = pkgs.mkShell {
            nativeBuildInputs = [ treefmtStack.${system}.config.build.wrapper ];
            buildInputs = with pkgs; [
              go
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
