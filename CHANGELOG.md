# Changelog

All notable PeerBlade changes are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and releases use
semantic versioning.

Each release states separately whether the control plane, the agent, or the
deployment bundle changed. “Agent update required” refers to reinstalling the
agent from the server page after the panel has been updated.

## [Unreleased]

No changes yet.

## [0.5.2] - 2026-08-12

### Panel

- Added a removal guide to Settings without granting the panel access to the
  host Docker socket.

### Deployment

- Fixed cleanup of the temporary file used while issuing a first-run setup
  token.
- Added safe removal instructions for stopping PeerBlade, retaining its data,
  or permanently deleting the control plane.

### Agent

- No functional changes.

### Upgrade notes

- Agent update required: **no**.
- Database migration required: **no**.
- Update the panel with `docker compose pull && docker compose up -d`.

## [0.5.1] - 2026-08-12

### Panel

- Changed Deploy calls to action on the landing page to lead to the Deployment
  section while preserving the existing Open panel behavior.

### Agent

- No functional changes.

### Upgrade notes

- Agent update required: **no**.
- Database migration required: **no**.
- Update with `docker compose pull && docker compose up -d`.

## [0.5.0] - 2026-08-12

### Panel

- Added a one-time first-administrator setup link in place of the production
  Basic Auth step.
- Added realistic vector interface previews and refreshed the landing hero.
- Added the GitHub call to action and polished server, pagination, session and
  activity states.

### Deployment

- Setup tokens contain 256 random bits, are stored only as SHA-256 hashes and
  expire after 60 minutes.
- Added setup-link reissuing and preserved Basic Auth compatibility for older
  installations and the development contour.

### Agent

- No functional changes.

### Upgrade notes

- Agent update required: **no**.
- Database migration required: **no**.
- Existing configured installations remain compatible.

## [0.4.0] - 2026-08-10

### Panel

- Added one-command installation for the control plane and optional first node.
- Added the installation screen, connection diagram and mobile deployment
  layout.
- Completed the PeerBlade rename throughout the product and database.
- Added peer labels, traffic history, peer detail pages, pagination and
  additional panel accounts.

### Agent

- The installer can prepare a dedicated WireGuard interface, forwarding, NAT
  and firewall rules.
- Added the first-node provisioning flow.
- Added safe import support for compatible wg-easy state.
- Preserved compatibility with older agents reporting the legacy management
  type.

### Upgrade notes

- Agent update required: **recommended** to use managed-interface setup and
  import features.
- Database migration required: **yes, automatic** when the stack starts.

## [0.3.1] - 2026-08-10

### Deployment

- Corrected published image metadata and repository references.

### Upgrade notes

- Agent update required: **no**.
- Database migration required: **no**.

## [0.3.0] - 2026-08-08

### Panel

- Added peer labels and cross-node filtering.
- Added traffic buckets, charts and retention for today, yesterday, week and
  month views.
- Added peer detail pages with configuration, QR code and confirmed removal.
- Replaced duplicate peer lists with one registry and added pagination.
- Refreshed the brand, page headings, server cards and monitoring layout.
- Added FAQ, SEO metadata, legal pages and panel-aware landing actions.

### Agent

- No protocol-breaking changes.

### Upgrade notes

- Agent update required: **no**.
- Database migration required: **yes, automatic** for peer labels and traffic
  history.

## [0.2.0] - 2026-08-06

### Deployment

- Published API, web and agent artifacts for both `linux/amd64` and
  `linux/arm64`.
- Made the agent installer select the correct binary for the host architecture.

### Agent

- Added arm64 builds.

### Upgrade notes

- Agent update required: **only for arm64 installations**.
- Database migration required: **no**.

## [0.1.1] - 2026-08-06

### Deployment

- Made agent binaries and checksums available from every panel host rather than
  only the project panel domain.

### Upgrade notes

- Agent update required: **no**.
- Database migration required: **no**.

## [0.1.0] - 2026-08-05

### Panel

- First packaged PeerBlade control plane with servers, peers, monitoring,
  activity, authentication and panel accounts.
- Added safe server archiving, removal and agent reconnection flows.
- Added production and development deployment workflows.
- Added the public product site, localization, legal pages and project
  documentation.

### Agent

- Added native systemd agent enrollment, heartbeats, snapshots and management
  commands.
- Kept WireGuard peer private keys on the node.
- Added install, reconnect and verified removal commands.

### Upgrade notes

- Initial release.
