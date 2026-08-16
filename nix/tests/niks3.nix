{ pkgs, module, ... }:
pkgs.testers.runNixOSTest {
  name = "comin-niks3";
  sshBackdoor.enable = true;  
  nodes.machine = { ... }: {
    imports = [ module ];
    virtualisation.graphics = false;
    services.comin = {
      enable = true;
      fetcher = {
        type = "niks3";
        niks3.remotes = {
          main = {
            url = "http://example.com";
          };
        };
      };
    };
    system.stateVersion = "26.05";
  };
  testScript = ''
    start_all()

    machine.succeed("systemctl start comin")
    machine.wait_for_unit("comin.service")
    machine.succeed("systemctl is-active comin")
    machine.wait_until_succeeds("comin status", 10)
  '';
}
