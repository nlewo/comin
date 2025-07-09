## services\.comin\.enable



Whether to run the comin service\.



*Type:*
boolean



*Default:*
` false `



## services\.comin\.package



The comin package to use\.



*Type:*
null or package



*Default:*
` "pkgs.comin or comin.packages.\${system}.default or null" `



## services\.comin\.debug

Whether to run comin in debug mode\. Be careful, secrets are shown!\.



*Type:*
boolean



*Default:*
` false `



## services\.comin\.exporter



Options for the Prometheus exporter\.



*Type:*
submodule



*Default:*
` { } `



## services\.comin\.exporter\.listen_address



Address to listen on for the Prometheus exporter\. Empty string will listen on all interfaces\.



*Type:*
string



*Default:*
` "" `



## services\.comin\.exporter\.openFirewall



Open port in firewall for incoming connections to the Prometheus exporter\.



*Type:*
boolean



*Default:*
` false `



## services\.comin\.exporter\.port



Port to listen on for the Prometheus exporter\.



*Type:*
signed integer



*Default:*
` 4243 `



## services\.comin\.flakeSubdirectory



Subdirectory in the repository, containing flake\.nix\.



*Type:*
string



*Default:*
` "." `



## services\.comin\.gpgPublicKeyPaths



A list of GPG public key file paths\. Each of this file should contains an armored GPG key\.



*Type:*
list of string



*Default:*
` [ ] `



## services\.comin\.hostname



The name of the configuration to evaluate and deploy\.
This value is used by comin to evaluate the flake output
nixosConfigurations\.“\<hostname>” or darwinConfigurations\.“\<hostname>”\.
Defaults to networking\.hostName - you MUST set either this option
or networking\.hostName in your configuration\.



*Type:*
string



*Default:*
` "the-machine-hostname" `



## services\.comin\.machineId



The expected machine-id of the machine configured by
comin\. If not null, the configuration is only deployed
when this specified machine-id is equal to the actual
machine-id\.
This is mainly useful for server migration: this allows
to migrate a configuration from a machine to another
machine (with different hardware for instance) without
impacting both\.
Note it is only used by comin at evaluation\.



*Type:*
null or string



*Default:*
` null `



## services\.comin\.postDeploymentCommand



A path to a script executed after each
deployment\. comin provides to the script the following
environment variables: ` COMIN_GIT_SHA `, ` COMIN_GIT_REF `,
` COMIN_GIT_MSG `, ` COMIN_HOSTNAME `, ` COMIN_FLAKE_URL `,
` COMIN_GENERATION `, ` COMIN_STATUS ` and ` COMIN_ERROR_MSG `\.



*Type:*
null or absolute path



*Default:*
` null `



*Example:*

```
pkgs.writers.writeBash "post" "echo $COMIN_GIT_SHA";

```



## services\.comin\.remotes



Ordered list of repositories to pull\.



*Type:*
list of (submodule)



## services\.comin\.remotes\.\*\.auth



Authentication options\.



*Type:*
submodule



*Default:*
` { } `



## services\.comin\.remotes\.\*\.auth\.access_token_path



The path of the auth file\.



*Type:*
string



*Default:*
` "" `



## services\.comin\.remotes\.\*\.branches



Branches to pull\.



*Type:*
submodule



*Default:*
` { } `



## services\.comin\.remotes\.\*\.branches\.main



The main branch to fetch\.



*Type:*
submodule



*Default:*
` { } `



## services\.comin\.remotes\.\*\.branches\.main\.name



The name of the main branch\.



*Type:*
string



*Default:*
` "main" `



## services\.comin\.remotes\.\*\.branches\.testing



The testing branch to fetch\.



*Type:*
submodule



*Default:*
` { } `



## services\.comin\.remotes\.\*\.branches\.testing\.name



The name of the testing branch\.



*Type:*
string



*Default:*
` "testing-the-machine-hostname" `



## services\.comin\.remotes\.\*\.name



The name of the remote\.



*Type:*
string



## services\.comin\.remotes\.\*\.poller



The poller options\.



*Type:*
submodule



*Default:*
` { } `



## services\.comin\.remotes\.\*\.poller\.period



The poller period in seconds\.



*Type:*
signed integer



*Default:*
` 60 `



## services\.comin\.remotes\.\*\.timeout



Git fetch timeout in seconds\.



*Type:*
signed integer



*Default:*
` 300 `



## services\.comin\.remotes\.\*\.url



The URL of the repository\.



*Type:*
string


