# PeerBlade Agent

The PeerBlade Agent is the open-source node component of PeerBlade. It runs on
a Linux WireGuard node, connects outbound to a PeerBlade control plane and
applies peer operations through the kernel WireGuard API.

The agent is licensed under the
[GNU General Public License v3.0 or later](LICENSE). The PeerBlade web panel and
control-plane API are separate proprietary programs and are not covered by this
license.

## What the agent does

- registers a node and sends periodic heartbeats over HTTPS;
- reads WireGuard interfaces through `wgctrl` and uploads safe snapshots;
- creates, enables, disables and removes peers on the dedicated managed
  interface;
- generates peer private and preshared keys locally;
- stores managed peer secrets in a `0600` state file on the node;
- polls the control plane for authenticated peer-management commands.

It does not proxy VPN traffic, open an inbound management port, use SSH, read
the Docker socket or send private keys in snapshots.

See [PROTOCOL.md](PROTOCOL.md) for the HTTP exchange and [SECURITY.md](SECURITY.md)
for privileges, local secrets and vulnerability reporting.

## Requirements

- Linux on amd64 or arm64;
- Go 1.23 or newer to build from source;
- a kernel WireGuard interface;
- `CAP_NET_ADMIN` to inspect and configure WireGuard through netlink;
- outbound HTTPS access to a compatible PeerBlade control plane.

## Build and test

```bash
cd agent
go test ./...
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -trimpath \
  -ldflags "-s -w -X main.agentVersion=0.6.1" \
  -o dist/peerblade-agent-linux-amd64 .
```

Release tags use the form `agent-vX.Y.Z`. The public workflow builds amd64 and
arm64 binaries, publishes SHA-256 checksums and creates build-provenance
attestations. The version printed by `peerblade-agent version` must match the
tag without the `agent-v` prefix.

Agent versions are independent from control-plane releases. See the
[agent changelog](CHANGELOG.md) for compatibility and upgrade notes.

## Install

The recommended installation command is issued by the server page of your own
PeerBlade control plane. It contains a short-lived enrollment token and selects
the correct release binary for the node architecture.

The scripts used by that command are published in [`deploy/`](deploy/) so they
can be reviewed before being run as root. Never paste an enrollment command or
the resulting `/etc/peerblade/agent.env` into a public issue.

## Compatibility

The control plane advertises the recommended agent version in its UI. Older
agents remain compatible while their protocol version is supported, but should
be upgraded when the panel reports an available update.

## Contributing

Small, focused pull requests are welcome. Read
[CONTRIBUTING.md](CONTRIBUTING.md) before opening one. Contributions are
licensed under GPL-3.0-or-later.
