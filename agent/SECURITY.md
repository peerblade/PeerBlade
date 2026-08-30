# Security policy

## Reporting a vulnerability

Please report suspected vulnerabilities privately to
`security@peerblade.com`. Do not include agent tokens, enrollment commands,
private keys, preshared keys or complete configuration files in a public issue.

Include the affected agent version, operating system, architecture and the
smallest reproduction that does not expose secrets. We will acknowledge the
report, investigate it and coordinate disclosure before publishing details.

## Privilege model

The systemd service runs as the dedicated `peerblade` user. It receives
`CAP_NET_ADMIN`, which is required by the Linux WireGuard netlink API and the
AmneziaWG control interface, and uses
a hardened unit with `NoNewPrivileges`, a restricted filesystem, namespaces,
devices, address families and system-call architecture.

The agent does not receive the Docker socket and does not require SSH access.
Its only network-management connection is outbound HTTP(S) to the configured
control-plane URL. Production deployments must use HTTPS.

## Secrets

- The agent token is stored in `/etc/peerblade/agent.env` with mode `0600`.
- Managed peer private and preshared keys are stored below
  `/var/lib/peerblade-agent` with directory mode `0700` and file mode `0600`.
- State writes use a temporary file, `fsync` and an atomic rename.
- Snapshots contain public operational state, not private or preshared keys.

Anyone with root access to the node can still read these files and control the
managed interface. PeerBlade does not attempt to protect a node from its own
root administrator.

## Scope

This policy covers the source and release artifacts in the `agent/` directory.
For control-plane or website vulnerabilities, use the same private address and
identify the affected PeerBlade release.
