# Suggested Commands
- `./cmd.sh run --config <path>` – runs the CLI directly with the default build tags after ensuring `go mod tidy` has run; accepts any command defined under `cmd/` (e.g., `extension`, `warp`).
- `make linux-custom` – fast local build of the `HiddifyCli` binary using the default tag set; outputs to `bin/` and refreshes the bundled web UI.
- `make linux-amd64 | make windows-amd64 | make macos-universal | make android | make ios` – platform-specific builds that create `libcore` shared libraries or gomobile artifacts under `bin/`.
- `make protos` – regenerates Go and grpc-web stubs from the protobuf definitions in `hiddifyrpc/` and bundles the browserified RPC client.
- `go test ./...` – runs the Go unit tests (currently concentrated in `extension/ui`); use `go test ./path/to/pkg -run TestName` for targeted checks.
- `npm install` – installs the JS dependencies needed for the extension web UI and grpc-web bundling; required before running `make protos` or `lib_install`.
