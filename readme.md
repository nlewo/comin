# Comin

Comin is a deployment tool polling a Git repository and deploying the NixOS
configuration found in this repository on the machine where it is executed.


By default, `comin` tracks the `main` branch of the repository. When
new commits are push to this branch, `comin` pulls them and run
`nixos-rebuild switch`. If this branch is hard reset, `comin` then
refuses to deploy the `main` branch.


## How to test a configuration

The `testing` branch of the repository is on top of the `main` branch,
Comin deploys this configuration by running `nixos-rebuild test`: the
bootloader is not modified.

So, to test a configuration:

1. Create a `testing` branch in your configuration repository on top of the `main` branch
2. Add new commits to this branch
3. Comin runs `nixos-rebuild test` on the found configuration

Note this branch can be hard reset and Comin then deploys the hard
reset configuration.

To `nixos-rebuild switch` to the testing configuration, the `main`
branch has to be rebased on the `testing` branch.
