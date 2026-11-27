{
  description = "Go formatter that compacts eligible three-line blocks";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs }:
    let
      inherit (nixpkgs) lib;
      systems = [ "aarch64-darwin" "aarch64-linux" "x86_64-darwin" "x86_64-linux" ];
      src = lib.fileset.toSource {
        root = ./.;
        fileset = lib.fileset.unions [
          ./go.mod
          ./go.sum
          ./format.go
          ./format_test.go
          (lib.fileset.fileFilter (file: file.hasExt "go") ./cmd)
          ./testdata
        ];
      };
    in
    {
      devShells = lib.genAttrs systems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            packages = [ pkgs.go self.packages.${system}.goheresy ];
          };
        });

      packages = lib.genAttrs systems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          goheresy = pkgs.buildGoModule {
            pname = "goheresy";
            version = "0.0.0";
            inherit src;
            vendorHash = "sha256-/dP8uA1yWBrtmFNHUvcicPhA2qr5R2v1uSwYi+ciypg=";
            nativeBuildInputs = [ pkgs.makeWrapper ];
            nativeCheckInputs = [ pkgs.diffutils ];
            allowGoReference = true;
            preCheck = ''
              export HOME="$TMPDIR/test-home"
              mkdir -p "$HOME"
            '';
            postInstall = ''
              wrapProgram "$out/bin/goheresy" \
                --suffix PATH : ${lib.makeBinPath [ pkgs.go pkgs.diffutils ]}
            '';

            meta.mainProgram = "goheresy";
          };
        in
        {
          inherit goheresy;
          default = goheresy;
        });
    };
}
