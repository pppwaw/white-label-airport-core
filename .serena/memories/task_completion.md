# Task Completion Checklist
- Format Go files with `gofmt -w` (or `goimports`) before committing; ensure generated code or build tags remain intact.
- Run `go test ./...` to make sure existing packages still pass; add focused package runs for any new code.
- If protobufs/extension assets were touched, rerun `make protos` (or the relevant Make target) and include the generated files.
- Rebuild the CLI or target artifact you affected (`make linux-custom`, `make android`, etc.) to confirm it still compiles and produces outputs under `bin/`.
- For extension/UI changes, rerun `npm install` if dependencies changed and rebuild the RPC bundle so CI will pass.
- Verify git status stays clean aside from intentional changes and reference related issues in the commit/PR body.
