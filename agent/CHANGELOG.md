# Agent changelog

Agent releases use independent `agent-vX.Y.Z` tags. The version shown in the
PeerBlade node page is the agent version, which does not have to match the
control-plane image version.

## [Unreleased]

## [0.7.1] - 2026-08-30

- Fixed AmneziaWG snapshot parsing when `awg show dump` reports disabled
  persistent keepalive as `off` instead of a numeric zero.

## [0.7.0] - 2026-08-29

- Added AmneziaWG peer creation, enable/disable/delete operations and client
  configuration generation through the official `awg` control interface.
- Added AmneziaWG snapshots with handshakes and RX/TX counters to the same
  protocol used for WireGuard.
- Added transport-aware runtime configuration. Environments without
  `PEERBLADE_MANAGED_TRANSPORT` continue to use WireGuard.

## [0.6.2] - 2026-08-28

- Added the `peer_traffic` capability and lightweight ten-second WireGuard
  counter reports for current RX/TX speed in the control plane.

## [0.6.1] - 2026-08-21

- First public source release under GPL-3.0-or-later.
- Published the Go source, tests, node-side deployment scripts, protocol and
  security documentation.
- Added public Linux amd64 and arm64 builds with SHA-256 checksums and
  build-provenance attestations.

There is no WireGuard migration associated with this publication. Existing
agent `0.6.1` installations already run the same implementation.
