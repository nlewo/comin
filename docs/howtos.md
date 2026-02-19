## How to test a NixOS configuration change

TLDR: push a commit to the `testing-<hostname>` branch (rebased on the
`main` branch) to deploy a change to the machine named `<hostname>`.

By default, each machine pulls configuration from the branch
`testing-<hostname>`. When this branch is on top of the `main` branch,
comin deploys this configuration by running `switch-to-configuration
test`: the bootloader configuration is not modified.

To test a configuration:

1. Create a `testing-<hostname>` branch in your configuration
   repository on top of the `main` branch
2. Add new commits to this branch and push it
3. comin runs `switch-to-configuration test` on the configuration: the bootload is not updated

Contrary to the main branch, this branch can be hard reset but always
has to be on top of the `main` branch.

To `nixos-rebuild switch` to this configuration, the `main` branch has
to be rebased on the `testing` branch.

## Iterate faster with local repository

By default, comin polls remotes every 60 seconds. You could however
add a local repository as a comin remote: comin could then poll this
branch every second. When you commit to this repository, comin is
starting to deploy the new configuration immediately.

However, be careful because this repository could then be used by an
attacker to update your machine.

Example of a configuration with a local repository:

```nix
services.comin = {
  enable = true;
  remotes = [
    {
      name = "local";
      url = "/your/local/infra/repository";
      poller.period = 2;
    }
  ];
}
```

## How to migrate a configuration from a machine to another one

Suppose you have a running NixOS machine and you want to move this
configuration to another machine while preserving the same
hostname. If you use a testing branch, both of these machines will be
updated when changes are pushed to the testing branch. 

To avoid such situation, we could set the option
`services.comin.machineId`. If the machine where comin is running
doesn't have this expected `machine-id` (compared to the content of
the `/etc/machine-id` file), comin won't deploy the configuration.

So, to migrate to another machine, you have to update this
option in the `testing-<hostname>` branch in order to only deploy this
configuration to the new machine.

## Check Git commit signatures

The option `services.comin.gpgPublicKeyPaths` allows to declare a list
of GPG public keys. If `services.comin.gpgPublicKeyPaths != []`, comin **only** evaluates commits signed
by one of these GPG keys. Note only the last commit needs to be signed.

The file containing a GPG public key has to be created with `gpg --armor  --export alice@cyb.org`.


## How to deploy a nix-darwin configuration

When comin is running on a Darwin system, it automatically builds and
deploys a configuration found in the flake output
`darwinConfigurations.hostname`. So, you only need to set this flake
output and run comin on the target machine.


## How to use comin without Nix flake

comin supports deploying configuration from a repository that doesn't
use Nix flake. Here is a configuration example:

```nix
services.comin = {
  repositoryType = "nix";
  systemAttr = "your.nixos.configuration.attribute";
  ...
  ...
}
```

Please note this is currently not supported by for nix-darwin configurations.

## Deployments and profiles retention policy

comin tracks deployments in its persisent storage. It uses this list
to add and remove system profiles which are consumed by bootloaders to install
boot menu entries.

The goal of the retention policy is to
1. keep the currently booted system, in order to always be able to rollback on reboot
2. keep the currently switched system
3. keep a configuration quantity of successful deployments leading to a boot menu entries (typically, `boot` and `switch` deployments).
   This is configuration thanks to `retention.keepBootEntries`
4. keep a configuration quantity of deployments to provide an history to the user
   This is configuration thanks to `retention.keepDeploymentEntries`

Thanks to this retention policy, you could then enable the
`nix.gc.automatic` module to automatically clean up your Nix store,
while preserving a configurable gcroot history.

Note that your bootloader entries shows one more entries than the ones
listed by `comin deployment list`. This is currently a comin
implementation limitation. Comin first creates a deployment, deploys it and
if it has been successfully deployed, it can remove older deployments,
accordingly to the retention policy. However, when it removes the
deployment, it currently doesn't reinstall the bootloader.

## What happen on /var/lib/comin deletion

comin store a state file in the `/var/lib/comin` directory. Here are
the consequencies:

- comin no longer knows the last deployed commit ID. It would then be
  possible for an attacker to hard reset the remote repository main
  branch. If you signed your commits, an attacker could then rollback
  the repository to a previous signed commit, which could contains
  CVEs.
- comin no longer knows the deployment history. On the next
  deployment, it would then no longer able to generate boot entries
  for previous deployments. However, if you delete the
  `/var/lib/comin` directory, the current booted entry would still be
  present in the boot menu.
