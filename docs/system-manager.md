## Deploying non-NixOS Linux machines with system-manager

comin supports [system-manager](https://github.com/numtide/system-manager)
as a deployment target, allowing you to manage non-NixOS Linux machines
with GitOps. Instead of evaluating `nixosConfigurations.<hostname>`,
comin evaluates `systemConfigs.<hostname>` and uses `system-manager-engine`
to register and activate the configuration.

### Prerequisites

- A non-NixOS Linux machine with Nix installed
- [system-manager](https://github.com/numtide/system-manager) available in your flake inputs
- A Git repository containing your system-manager configuration

### Quick start

#### 1. Set up your flake

Your `flake.nix` should expose a `systemConfigs.<hostname>` output and
import the comin system-manager module:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    system-manager.url = "github:numtide/system-manager";
    comin.url = "github:trycua/comin";
  };

  outputs = { self, nixpkgs, system-manager, comin, ... }:
  let
    # Adjust to your target architecture
    system = "x86_64-linux";
    pkgs = nixpkgs.legacyPackages.${system};
  in
  {
    systemConfigs.my-server = system-manager.lib.makeSystemConfig {
      modules = [
        # Import the comin module for system-manager
        comin.systemManagerModules.comin

        # Your machine configuration
        ./configuration.nix
      ];
    };
  };
}
```

#### 2. Configure comin in your system-manager configuration

In `configuration.nix`:

```nix
{ pkgs, lib, ... }:
{
  services.comin = {
    enable = true;
    # Required: system-manager has no networking.hostName,
    # so the hostname must be set explicitly.
    hostname = "my-server";
    remotes = [
      {
        name = "origin";
        url = "https://github.com/your-org/your-infra.git";
      }
    ];
  };

  # Your other system-manager configuration...
  environment.systemPackages = [ pkgs.htop ];
}
```

#### 3. Bootstrap the first deployment

On the target machine, clone your repository and run comin manually for the
first time:

```bash
# Clone the configuration repository
git clone https://github.com/your-org/your-infra.git /tmp/infra

# Run comin once to bootstrap
comin run --config /tmp/comin.yaml
```

After the first deployment, the comin systemd service is installed and
starts polling the remote repository automatically.

### How deployment works

When comin detects a new commit, it performs these steps:

1. **Evaluate** - runs `nix derivation show` on `systemConfigs."<hostname>"` from your flake
2. **Build** - runs `nix build` on the derivation
3. **Register** - runs `system-manager-engine register --store-path <out>` to create a nix profile generation (skipped for `test` operations)
4. **Activate** - runs `system-manager-engine activate --store-path <out>` to apply etc files and start services

Unlike NixOS, system-manager deployments never require a reboot.

### Configuration options

The `hostname` option is required and must be set explicitly (unlike the
NixOS module which defaults to `networking.hostName`):

```nix
services.comin = {
  enable = true;
  hostname = "my-server";  # Must match systemConfigs.<hostname>
  remotes = [ ... ];
};
```

All other options mirror the NixOS module:

| Option | Default | Description |
|--------|---------|-------------|
| `hostname` | *(required)* | Name matching `systemConfigs.<hostname>` in your flake |
| `remotes` | | List of Git remotes to poll |
| `repositorySubdir` | `"."` | Subdirectory containing `flake.nix` |
| `submodules` | `false` | Fetch Git submodules |
| `debug` | `false` | Enable debug logging |
| `gpgPublicKeyPaths` | `[]` | GPG public keys for commit signature verification |
| `postDeploymentCommand` | `null` | Script to run after each deployment |
| `machineId` | `null` | Expected machine-id (for migration safety) |
| `exporter.port` | `4243` | Prometheus exporter port |
| `buildConfirmer.mode` | `"without"` | Build confirmation mode (`without`, `auto`, `manual`) |
| `deployConfirmer.mode` | `"without"` | Deploy confirmation mode (`without`, `auto`, `manual`) |

### Examples

#### Private repository with access token

```nix
services.comin = {
  enable = true;
  hostname = "my-server";
  remotes = [
    {
      name = "origin";
      url = "https://github.com/your-org/private-infra.git";
      auth.access_token_path = "/run/secrets/github-token";
    }
  ];
};
```

#### Multiple remotes with a local fast-polling repository

```nix
services.comin = {
  enable = true;
  hostname = "my-server";
  remotes = [
    {
      name = "origin";
      url = "https://github.com/your-org/your-infra.git";
      branches.testing.name = "";  # No testing branch on remote
    }
    {
      name = "local";
      url = "/home/admin/infra";
      poller.period = 2;  # Poll every 2 seconds for fast iteration
    }
  ];
};
```

#### Testing branches

Like NixOS deployments, you can use testing branches to try configuration
changes before committing them to the main branch. Push to the
`testing-<hostname>` branch to deploy with the `test` operation (activate
only, no profile registration):

```bash
git checkout -b testing-my-server
# Make your changes
git push origin testing-my-server
```

The testing branch must be on top of the `main` branch. When you are
satisfied with the change, merge it into `main` to make it permanent.

#### GPG commit signature verification

```nix
services.comin = {
  enable = true;
  hostname = "my-server";
  remotes = [
    {
      name = "origin";
      url = "https://github.com/your-org/your-infra.git";
    }
  ];
  gpgPublicKeyPaths = [
    ./keys/deployer.gpg
  ];
};
```

#### Deploy confirmation

Use the deploy confirmer to automatically rollback if a deployment breaks
connectivity:

```nix
services.comin = {
  enable = true;
  hostname = "my-server";
  remotes = [
    {
      name = "origin";
      url = "https://github.com/your-org/your-infra.git";
    }
  ];
  deployConfirmer = {
    mode = "auto";
    autoconfirm_duration = 120;  # Rollback if not confirmed within 120s
  };
};
```

### Differences from NixOS deployments

| | NixOS | system-manager |
|---|-------|---------------|
| Flake output | `nixosConfigurations.<host>` | `systemConfigs.<hostname>` |
| Deployment tool | `switch-to-configuration` | `system-manager-engine` |
| Profile location | `/nix/var/nix/profiles/system` | `/nix/var/nix/profiles/system-manager-profiles/system-manager` |
| Reboot support | Yes (detects kernel changes) | No (never requires reboot) |
| `hostname` option | Defaults to `networking.hostName` | Must be set explicitly |
| Systemd target | `multi-user.target` | `system-manager.target` |
| Module import | `comin.nixosModules.comin` | `comin.systemManagerModules.comin` |

### Using comin YAML config directly

If you are not using the comin Nix module and want to configure comin via
YAML, set `repository_type` to `"system-manager"`:

```yaml
hostname: my-server
state_dir: /var/lib/comin
repository_type: system-manager
remotes:
  - name: origin
    url: https://github.com/your-org/your-infra.git
    branches:
      main:
        name: main
        operation: switch
      testing:
        name: testing-my-server
        operation: test
    poller:
      period: 60
```
