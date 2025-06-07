# Authentication for private repositories

## Access token

### GitLab

You need to create a [personal access token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html) with the `read_repository` scope
and store this token into a file (`/filepath/to/your/access/token` in example 1). 

### GitHub

You need to create a [fined-grained personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens#fine-grained-personal-access-tokens)
and store this token into a file (`/filepath/to/your/access/token` in
example 1).
Note: classic personal access tokens are also supported.

### Use your personal access token

The file path containing this access token for a remote is provided
with the attribute `comin.remotes.*.auth.access_token_path`.

#### Example 1 - access token

```nix
services.comin = {
  enable = true;
  remotes = [
    {
      name = "origin";
      url = "https://gitlab.com/your/private-infra.git";
      auth.access_token_path = "/filepath/to/your/access/token";
    }
  ];
};
```

## SSH

You need to create a [SSH deploy key](https://docs.github.com/en/authentication/connecting-to-github-with-ssh/managing-deploy-keys) with access to the flake repository and any private inputs.
store this key into a file (`/filepath/to/your/deploy/key` in example 2).
Note: user access keys are also supported.

#### Example 2 - SSH

```nix
systemd.services.comin = {
  environment = {
    GIT_SSH_COMMAND = "${pkgs.openssh}/bin/ssh -i /filepath/to/your/deploy/key";
  };
};
```
