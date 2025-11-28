## How to run comin locally

     $ go build
	 $ sudo ./comin run --config ./internal/config/configuration.yaml --debug

You need to update the `configuration.yaml` file with your remotes. 
It is also possible to use the YAML configuration file generation by
your module (see `systemctl show comin.service | grep ExecStart=`).
