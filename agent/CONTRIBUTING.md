# Contributing to the PeerBlade Agent

Contributions are welcome for the code and node-side scripts under `agent/`.
The proprietary web panel and control-plane API are outside this repository's
open-source contribution scope.

Before opening a pull request:

1. Keep the change focused and describe the operational reason for it.
2. Run `gofmt` on changed Go files.
3. Run `go test ./...` and `go vet ./...` from the `agent` directory.
4. Run `bash -n deploy/*.sh scripts/*.sh` for shell changes.
5. Do not include agent tokens, enrollment commands, private keys, real node
   addresses or production configuration in code, tests, issues or logs.

Protocol changes should update `PROTOCOL.md` and preserve compatibility with
supported control-plane releases. Security issues must be reported privately
as described in `SECURITY.md` rather than opened as a public issue.

By submitting a contribution, you license it under GPL-3.0-or-later, the same
license as the agent.
