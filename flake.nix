{
  description = "vimv2";
  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    nixpkgs.url = "nixpkgs/nixpkgs-unstable";
  };
  outputs = { self, flake-utils, nixpkgs }:
    flake-utils.lib.eachDefaultSystem
      (system:
        with import nixpkgs { inherit system; }; rec {
          packages.vimv2 = buildGo118Module rec {
            name = "vimv2";
            pname = name;
            src = ./.;
            vendorSha256 = "iAe2jFjATyG+HV5YJiXKV4w1AfMep4do2tYGnuWOJ0E=";
          };
          defaultPackage = packages.vimv2;

          devShell = mkShell {
            nativeBuildInputs = [ go_1_18 gopls ];
          };
        }) // {
      overlay = (final: prev: { vimv2 = self.defaultPackage."${prev.system}"; });
    };
}
