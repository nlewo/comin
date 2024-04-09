# comin - GitOps for NixOS Machines

**comin** is a deployment tool operating in pull mode. Running on a
machine, it periodically polls Git repositories and deploys the NixOS
configuration associated to the machine.

## Features

- :snowflake: Git push to deploy NixOS configurations
- :construction: Support testing branches to [try changes](./docs/howtos.md#how-to-test-a-nixos-configuration-change)
- :rocket: Poll [multiple Git remotes](./docs/generated-module-options.md#servicescominremotes) to avoid SPOF
- :postbox: Support [machines migrations](./docs/howtos.md#how-to-migrate-a-configuration-from-a-machine-to-another-one)
- :fast_forward: Fast iterations with [local remotes](./docs/howtos.md#iterate-faster-with-local-repository)
- :satellite: Observable via [Prometheus metrics](./docs/generated-module-options.md#servicescominexporter)

## Quick start

In your `configuration.nix` file:

```nix
services.comin = {
  enable = true;
  remotes = [
    {
      name = "origin";
      url = "https://gitlab.com/your/infra.git";
    }
  ];
};
```

This enables a systemd service, which periodically pulls the `main`
branch of the repository `https://gitlab.com/your/infra.git` and
deploys the NixOS configuration corresponding to the machine hostname.

A new commit in the `main` branch of the repository
`https://gitlab.com/your/infra.git` is then deployed in the next 60
seconds.

## Documentation

- [Howtos](./docs/howtos.md)
- [Advanced Configuraion](./docs/advanced-config.md)
- [Authentication](./docs/authentication.md)
- [Comin module options](./docs/generated-module-options.md)
- [Design](./docs/design.md)
