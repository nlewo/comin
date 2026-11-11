{
  inputs.comin.url = "../.";
  outputs =
    {
      self,
      comin,
      ...
    }:
    {
      nixosConfigurations.machine = comin.checks.x86_64-linux.comin-test.nodes.machine;
      nixosConfigurations.lanzaboote = comin.checks.x86_64-linux.lanzaboote-test.nodes.lanzaboote;
    };
}
