# Comin

Comin is a deployment tool running in the pull mode: it periodically
polls a Git repository and deploys the NixOS configuration found in
this repository on the machine where it is running.

## Getting started

In your `configuration.nix` file:

    services.comin = {
      enable = true;
      repository = "https://gitlab.com/your/infra.git";
    };

This enables a systemd service, which periodically pulls the `main`
branch of the repository `https://gitlab.com/your/infra.git` and
deploys the NixOS configuration corresponding to the machine hostname.

## Bootstrap Comin

Deploying your configuration on a new NixOS machine can be pretty
tedious. The `comin bootstrap` command allows to easily bootstrap
Comin. It pulls a repository and deploys the configuration.

    comin bootstrap <YOUR-REPOSITORY> <A-COMMIT-ID>

Note the commit ID is required to securely initialize Comin since it
garantees you are deploying what you expect.

## How to test a configuration without having to commit to main

By default, each machine also pulls configuration from the branch
`testing-<hostname>`. When this branch is on top of the `main` branch,
Comin deploys this configuration by running `switch-to-configuration
test`: the bootloader configuration is not modified.

To test a configuration:

1. Create a `testing-<hostname>` branch in your configuration
   repository on top of the `main` branch
2. Add new commits to this branch and push it
3. Comin runs `switch-to-configuration test` on the  configuration

Contrary to the main branch, this branch can be hard reset but always
has to be on top of the `main` branch.

To `nixos-rebuild switch` to the testing configuration, the `main`
branch has to be rebased on the `testing` branch.

## Authentication for private repositories

### GitLab

Currently, only the GitLab [personnal access
token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)
is supported.

- The module option `services.comin.authFile` allows to specify the
  path of a file containing the GitLab access token.
- The command `comin boostrap --ask-for-gitlab-access-token` allows to
  ask for a GitLab access token to fetch the repository.

## How to migrate a configuration from a machine to another one

When the option `services.comin.machineId` is set, Comin only deploys
the configuration on the machine if the option value matches the
`/etc/machine-id` value.

To migrate to another machine, it is then possible to update this option in the `testing-<hostname>` branch in order to only deploy this configuration on a the new machine.
