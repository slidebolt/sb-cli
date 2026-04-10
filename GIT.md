# Git Workflow for sb-cli

This repository contains the Slidebolt CLI tool (`sb`), used for managing the Slidebolt ecosystem from the command line. It produces a standalone binary.

## Dependencies
- **Internal:**
  - `sb-domain`: Shared domain models for entities and commands.
  - `sb-messenger-sdk`: Shared messaging interfaces.
  - `sb-script`: Scripting engine (used for validating or running local scripts).
  - `sb-storage-sdk`: Shared storage interfaces.
  - `sb-storage-server`: Storage implementation for local workspace operations.
- **External:** 
  - Standard Go library and NATS.

## Build Process
- **Type:** Go Application (CLI).
- **Consumption:** Run as a developer and operator tool.
- **Artifacts:** Produces a binary named `sb`.
- **Command:** `go build -o sb ./cmd/sb-cli`
- **Validation:** 
  - Validated through unit tests: `go test -v ./...`
  - Validated by successful compilation of the binary.

## Pre-requisites & Publishing
As the primary CLI tool, `sb-cli` must be updated whenever any of the core domain, messaging, storage, or scripting SDKs are changed.

**Before publishing:**
1. Determine current tag: `git tag | sort -V | tail -n 1`
2. Ensure all local tests pass: `go test -v ./...`
3. Ensure the binary builds: `go build -o sb ./cmd/sb-cli`

**Publishing Order:**
1. Ensure all internal dependencies are tagged and pushed.
2. Update `sb-cli/go.mod` to reference the latest tags.
3. Determine next semantic version for `sb-cli` (e.g., `v1.0.4`).
4. Commit and push the changes to `main`.
5. Tag the repository: `git tag v1.0.4`.
6. Push the tag: `git push origin main v1.0.4`.

## Update Workflow & Verification
1. **Modify:** Update CLI commands or logic in `app/` or `cmd/`.
2. **Verify Local:**
   - Run `go mod tidy`.
   - Run `go test ./...`.
   - Run `go build -o sb ./cmd/sb-cli`.
3. **Commit:** Ensure the commit message clearly describes the CLI change.
4. **Tag & Push:** (Follow the Publishing Order above).
