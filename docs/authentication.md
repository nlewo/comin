## Authentication for private repositories

### GitLab

Currently, only the GitLab [personnal access
token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)
is supported. The file path containing the access tokenfor a remote is
provided with the attribute `comin.remotes.*.auth.access_token_path`.

Example:

```nix
services.comin = {
  enable = true;
  remotes = [
    {
      name = "origin";
      url = "https://gitlab.com/your/private-infra.git";
      auth.access_token_path = "/filepath/to/your/access/token"
    }
  ];
]
```
