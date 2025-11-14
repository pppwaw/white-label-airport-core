# Code Style & Conventions
- Go sources must compile with Go 1.23+ and follow standard `gofmt`/`goimports` formatting. Keep build tags in sync with the Makefile defaults (`with_gvisor, with_quic, ...`) so new files build correctly across every platform.
- CLI commands live in `cmd/` as Cobra commands registered inside `init()`; keep command names short, verbs in lowercase, and expose only intentional flags. Shared structs/interfaces used by `custom/` or `mobile/` should stay in those packages to avoid circular imports.
- Extension code uses the SDK under `extension/sdk` and HTML UI assets in `extension/html`; when touching JS files, run Prettier (see `.prettierrc`) to keep quoting consistent.
- Protobuf changes must preserve package names and field numbers; regenerate Go and grpc-web stubs with `make protos` and commit the generated files.
