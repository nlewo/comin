{ pkgs, module }: {
  git-deprecated = import ./git-deprecated.nix { inherit pkgs module; };
  git = import ./git.nix { inherit pkgs module; };
  niks3 = import ./niks3.nix { inherit pkgs module; };
}
