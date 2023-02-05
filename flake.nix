{
  description = "Tailscale WHOIS service binding on localhost port for authentication proxy.";

  inputs.nixpkgs.url = "nixpkgs/nixos-22.11";

  outputs = { self, nixpkgs }: 
  let
      # Use date as simple version.
      version = builtins.substring 0 8 self.lastModifiedDate;

      # System types to support.
      supportedSystems = [ "x86_64-linux" "aarch64-linux" ];

      darwin = [ "x86_64-darwin" "aarch64-darwin" ];
      linux = [ "x86_64-linux" "aarch64-linux" ];

      forEachSystem = systems: f: nixpkgs.lib.genAttrs systems (system: f system);
      forAllSystems = forEachSystem (darwin ++ linux);

      # Nixpkgs instantiated for supported system types.
      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });

  in
  {
    packages = forAllSystems (system:
    let
      pkgs = nixpkgsFor.${system};
    in
    {
      ts-whois = pkgs.buildGoModule {
        pname = "ts-whois";
        inherit version;
        src = ./.;
        vendorHash = null;
      };
    });
    apps = forAllSystems (system: {
      default = {
        type = "app";
        program = "${self.packages.${system}.ts-whois}/bin/ts-whois";
      };
    });
    defaultPackage = forAllSystems (system: self.packages.${system}.ts-whois);
  };
}
