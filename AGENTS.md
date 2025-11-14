# Repository Guidelines

## Project Structure & Module Organization
CLI `cli/main.go` hands off to Cobra commands in `cmd/` (`run`, `warp`, `config`, `extension`). `custom/` builds shared libraries for desktop hosts, `mobile/` contains mobile bindings, and `extension/` hosts SDK, HTML demo, and server. Protobuf contracts stay in `hiddifyrpc/` (mirrored in `extension/html/rpc`); packaging assets live in `docker/`, `wrt/`, `bridge/`, builds land in `bin/`, and configs live in `config/`, `utils/`, `v2/`.

## Build, Test & Development Commands
- `./cmd.sh run --config ./config/default.json` executes the CLI with the default build tags; swap in other subcommands such as `warp` or `config`.
- `./cmd.sh extension` starts the self-hosted extension UI at `https://127.0.0.1:12346`.
- `make linux-custom` gives a quick local `HiddifyCli` build; use the platform variants (`make linux-amd64`, `make windows-amd64`, `make macos-universal`, `make android`, `make ios`) before distributing binaries.
- `pnpm install --frozen-lockfile && make protos` refreshes grpc-web bundles after editing `hiddifyrpc/*.proto` or `extension/html`.
- `go test ./...` (or `go test ./extension/ui -run TestFormUnmarshalJSON`) validates Go changes.

## Coding Style & Naming Conventions
Format every Go file with `gofmt`/`goimports`, keep Go 1.23+ compatibility, and apply the default tag set (`with_gvisor,with_quic,...`) to newly added sources. Packages stay lowercase, exported APIs (notably in `custom/` and `extension/sdk`) start with a capital letter, and Cobra commands remain short imperatives. JavaScript and HTML assets should pass Prettier (respect the `.prettierrc` single-quote rule), and protobufs must retain package names plus field numbers to avoid breaking clients.

## Testing Guidelines
Run `go test ./...` prior to every push, add focussed unit tests for any package you touch, and prefer table-driven coverage for config parsing or validation. Exercise UI or extension flows via `./cmd.sh extension` and capture the manual scenario in your PR notes. If you modify runtime configs under `v2/`, add regression tests so `cmd run` remains stable.

## Commit & Pull Request Guidelines
Write short, imperative commit subjects (`bump linux version` style) and reserve `release: version` for CI-controlled releases. Each PR should link an issue, describe what changed, and summarize verification (e.g., `go test ./...`, `make linux-custom`, `./cmd.sh extension`). Include screenshots or command output for UI/tunnel adjustments and keep generated artifacts (`make protos`, `bin/webui`) in the same diff when relevant.

## Security & Configuration Tips
Never commit customer configs, certificates, or Warp keys—start from the sanitized samples in `config/` or supply values through env vars. When touching Docker or WRT assets, keep default ports, placeholders, and upgrade hashes intact so downstream white-label builds stay secure.
