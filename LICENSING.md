# PeerBlade licensing

PeerBlade uses a split-source model. The license depends on the component:

- [`agent/`](agent/) — GNU General Public License v3.0 or later. This includes
  the Go node agent, its tests and the node-side deployment scripts stored in
  that directory.
- PeerBlade web panel and control-plane API container images — proprietary.
  Publishing deployment files in this repository does not grant a license to
  their source code or broaden the rights provided with those images.
- PeerBlade names, logos and other brand assets — not licensed for reuse by the
  agent's GPL license.

The agent and the control plane are published and distributed as separate
programs. They communicate over the documented HTTP interface in
[`agent/PROTOCOL.md`](agent/PROTOCOL.md).

The product terms cannot reduce or replace any rights granted for the agent by
GPL-3.0-or-later. For the exact agent license, see [`agent/LICENSE`](agent/LICENSE).
