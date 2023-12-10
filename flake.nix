{
  description = "vimv2";

  inputs = {
    nixpkgs.url = "nixpkgs/nixpkgs-unstable";
    utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, utils }: {
    overlays.default = final: _: {
      vimv2 = final.buildGoModule rec {
        name = "vimv2";
        pname = name;
        src = ./.;
        vendorHash = null;
      };
    };
  } // utils.lib.eachDefaultSystem (system: with import nixpkgs
    {
      overlays = [ self.overlays.default ]; inherit system;
    }; {
    packages.default = vimv2;

    devShells.default = mkShell {
      packages = [ go gopls ];
    };
  });
}
