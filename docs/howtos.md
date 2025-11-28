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
