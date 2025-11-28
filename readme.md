# comin - GitOps for NixOS Machines

**comin** is a NixOS deployment tool operating in pull mode. Running
on a machine, it periodically polls Git repositories and deploys the
NixOS configuration associated to the machine.

## Features

- :snowflake: Git push to deploy NixOS configurations (or [nix-darwin](./docs/howtos.md#how-to-deploy-a-nix-darwin-configuration))
- :construction: Support testing branches to [try changes](./docs/howtos.md#how-to-test-a-nixos-configuration-change)
- :rocket: Poll [multiple Git remotes](./docs/generated-module-options.md#servicescominremotes) to avoid SPOF
- :postbox: Support [machines migrations](./docs/howtos.md#how-to-migrate-a-configuration-from-a-machine-to-another-one)
- :fast_forward: Fast iterations with [local remotes](./docs/howtos.md#iterate-faster-with-local-repository)
- :satellite: Observable via [Prometheus metrics](./docs/generated-module-options.md#servicescominexporter)
- :pushpin: Create and delete system profiles
- :lock: Optionally check [Git commit signatures](./docs/howtos.md#check-git-commit-signatures)

## Quick start

This is a basic `flake.nix` example:

```nix
{
  inputs = {
    nixpkgs.url = "github:nixOS/nixpkgs";
    comin = {
      url = "github:nlewo/comin";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };
  outputs = { self, nixpkgs, comin }: {
    nixosConfigurations = {
      myMachine = nixpkgs.lib.nixosSystem {
        system = "x86_64-linux";
        modules = [
          comin.nixosModules.comin
          ({...}: {
            services.comin = {
              enable = true;
              remotes = [{
                name = "origin";
                url = "https://gitlab.com/your/infra.git";
                branches.main.name = "main";
              }];
            };
          })
        ];
      };
    };
  };
}
```

This enables a systemd service, which periodically pulls the `main`
branch of the repository `https://gitlab.com/your/infra.git` and
deploys the NixOS configuration corresponding to the machine hostname
`myMachine`.

A new commit in the `main` branch of the repository
`https://gitlab.com/your/infra.git` is then deployed in the next 60
seconds.

## Documentation

- [Howtos](./docs/howtos.md)
- [Advanced Configuraion](./docs/advanced-config.md)
- [Authentication](./docs/authentication.md)
- [Comin module options](./docs/generated-module-options.md)
- [Design](./docs/design.md)
- [Contribute](./docs/contribute.md)
