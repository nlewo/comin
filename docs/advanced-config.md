## Advanced configuration

```nix
services.comin = {
  enable = true;
  remotes = [
    {
      name = "origin";
      url = "https://gitlab.com/your/private-infra.git";
	    # This is an access token to access our private repository
      auth.access_token_path = cfg.sops.secrets."gitlab/access_token".path;
	    # No testing branch on this remote
      branches.testing.name = "";
    }
    {
      name = "local";
      url = "/your/local/infra/repository";
      # We don't want to deploy the local main branch on each commit
      branches.main.name = "main-tilia";
	    # We want to fetch this remote every 2 seconds
      poller.period = 2;
    }
  ];
  machineId = "22823ba6c96947e78b006c51a56fd89c";
};
```
