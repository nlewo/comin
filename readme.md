# comin - GitOps for NixOS Machines

**comin** is a deployment tool operating in pull mode. Running on a
machine, it periodically polls Git repositories and deploys the NixOS
configuration associated to the machine.

## Features

- Git push to deploy NixOS configurations
- Support testing branches to try changes
- Poll multiple Git remotes to avoid SPOF
- Support machines migrations
- Fast iterations with local remotes
- Observable thanks to exposed Prometheus metrics

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
- [Design](./docs/design.md)
