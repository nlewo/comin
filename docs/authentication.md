## Authentication for private repositories

### GitLab

You need to create a [personal access
token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html) with the `read_repository` scope
and store this token into a file (`/filepath/to/your/access/token` in the below example). 

### GitHub

You need to create a [fined-grained personal access
token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens#fine-grained-personal-access-tokens)
and store this token into a file (`/filepath/to/your/access/token` in
the below example). Note classic personal access tokens are also
supported.

### Use your personal access token

The file path containing this access token for a remote is provided
with the attribute `comin.remotes.*.auth.access_token_path`.

#### Example

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
