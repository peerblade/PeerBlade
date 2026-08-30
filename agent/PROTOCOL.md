# Agent protocol

The agent is an outbound HTTPS client. It does not listen on a management port.
Every request after enrollment uses an agent-specific bearer token stored in
`/etc/peerblade/agent.env` with mode `0600`.

## Enrollment

The installation command exchanges a short-lived, single-use enrollment token
for an agent token. The enrollment token is not retained after a successful
exchange. Reconnecting an existing installation rotates the agent token.

## Runtime endpoints

| Method | Path                                | Purpose                                                                            |
| ------ | ----------------------------------- | ---------------------------------------------------------------------------------- |
| `POST` | `/api/agent/v1/register`            | Associate the authenticated agent with its node and report version/capabilities.   |
| `POST` | `/api/agent/v1/heartbeat`           | Report liveness and the running agent version.                                     |
| `POST` | `/api/agent/v1/snapshot`            | Upload interfaces, peers, public keys, endpoints, handshakes and traffic counters. |
| `POST` | `/api/agent/v1/traffic`             | Upload lightweight peer counters used to derive current RX/TX speed.               |
| `GET`  | `/api/agent/v1/commands/next`       | Claim the next pending command for this agent.                                     |
| `POST` | `/api/agent/v1/commands/:id/result` | Acknowledge success or failure and return an optional command payload.             |

Snapshots deliberately exclude interface private keys, peer private keys and
preshared keys. A configuration requested by an authenticated administrator is
returned as the payload of a specific command; the control plane forwards it
to that administrator and does not persist it.

The traffic report contains only interface names, peer public keys and the same
cumulative byte counters already present in snapshots. It runs every ten
seconds by default and contains no endpoints, Allowed IPs or key material.

## Commands

The current native command set covers:

- create peer;
- enable peer;
- disable peer;
- remove peer;
- return a managed peer configuration.

Commands target the dedicated interface named in
`PEERBLADE_MANAGED_INTERFACE`; `PEERBLADE_MANAGED_TRANSPORT` selects the
WireGuard or AmneziaWG driver. Other interfaces may be included in read-only
snapshots but are not modified by native peer-management commands.

## Failure behaviour

Heartbeat, snapshot and command workers retry after temporary network or API
failures. The selected interface remains active with its last applied
configuration when the agent or control plane is unavailable; the agent is not
part of the data path.
