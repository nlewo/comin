{
  description = "Comin";

  outputs = { self, nixpkgs }:
  let
    system = "x86_64-linux";
    pkgs = import nixpkgs {
      system = "x86_64-linux";
      overlays = [ self.overlay ];
    };
  in {
    overlay = final: prev: {
      comin = pkgs.buildGoModule rec {
        pname = "comin";
        version = "0.0.1";
        # TODO: fix tests in sandbox :/
        doCheck = false;
        src = pkgs.lib.cleanSourceWith {
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
        vendorSha256 = "sha256-7P//MF0ZRDihabSgbxhqxilDrwhZPjMRGblZXBdNT2E=";
      };
    };
    packages.x86_64-linux.comin = pkgs.comin;
    defaultPackage.x86_64-linux = pkgs.comin;
    devShell.x86_64-linux = let
      pkgs = nixpkgs.legacyPackages.x86_64-linux;
    in pkgs.mkShell {
      buildInputs = [
        pkgs.go pkgs.godef
      ];
    };
  };
}
