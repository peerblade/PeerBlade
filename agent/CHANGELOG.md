# Agent changelog

Agent releases use independent `agent-vX.Y.Z` tags. The version shown in the
PeerBlade node page is the agent version, which does not have to match the
control-plane image version.

## [0.6.1] - 2026-08-21

- First public source release under GPL-3.0-or-later.
- Published the Go source, tests, node-side deployment scripts, protocol and
  security documentation.
- Added public Linux amd64 and arm64 builds with SHA-256 checksums and
  build-provenance attestations.

There is no WireGuard migration associated with this publication. Existing
agent `0.6.1` installations already run the same implementation.
