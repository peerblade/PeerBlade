# Changelog

All notable PeerBlade changes are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and releases use
semantic versioning.

Each release states separately whether the control plane, the agent, or the
deployment bundle changed. “Agent update required” refers to reinstalling the
agent from the server page after the panel has been updated.

## [Unreleased]

### Agent

- Published the complete Linux node-agent source, tests, installer scripts,
  protocol documentation and security model under GPL-3.0-or-later.
- Added a public amd64/arm64 release workflow with checksums and build
  provenance attestations.

### Documentation

- Clarified the split-source model: the node agent is open source while the
  web panel and control-plane API remain proprietary.

## [0.6.0] - 2026-08-17

### Panel

- Added an actionable notification when a node runs an older agent than the
  version bundled with the installed PeerBlade release.
- Notification read state is now stored per administrator in PostgreSQL and
  stays synchronized across browsers and devices.
- Server cards are displayed in a single column while retaining the existing
  compact expansion for larger node collections.
- Peer filters now wrap cleanly on narrow screens, disabled access switches
  use the neutral state colour, and peer creation uses a generic device name.

### Upgrade notes

- Agent update required: **no**.
- Database migration required: **yes**; it is applied automatically when the
  control plane starts.

## [0.5.9] - 2026-08-17

### Panel

- Installation receipts are now stored even when email delivery is not yet
  configured or is temporarily unavailable, and remain pending for a later
  notification attempt.
- Server cards now open from their full surface and use the same minimal
  icon-action treatment as the dashboard header.
- Peer lists now show the primary tunnel IP directly below the peer name.

### Website

- Updated the Interface preview with the clickable server-card treatment,
  minimal actions and fictional peer IP addresses used by the panel.

### Upgrade notes

- Agent update required: **no**.
- Database migration required: **no**.
- Update the panel with `docker compose pull && docker compose up -d`.

## [0.5.8] - 2026-08-13

### Website

- Updated the Interface preview to match the real dashboard header, including
  the notifications bell, unread indicator and settings gear.
- Changed the website contact address to `contact@peerblade.com`.
- Made the English privacy policy and terms the prevailing versions when the
  English and Russian texts differ.

### Panel

- Switched server collections larger than four nodes to a compact list that
  initially shows five entries and expands in place without hiding the
  connect-server form.

### Documentation

- Added `feedback@peerblade.com` as the public GitHub feedback address.

### Upgrade notes

- Agent update required: **no**.
- Database migration required: **yes**; it is applied automatically when the
  control plane starts.
- Update the panel with `docker compose pull && docker compose up -d`.

## [0.5.7] - 2026-08-13

### Agent

- Keeps heartbeat, snapshot and command polling workers alive across temporary
  control-plane or network failures and retries them automatically.
- Updated the systemd safety net to restart the agent indefinitely instead of
  leaving it stopped after five consecutive failures.

### Upgrade notes

- Agent update required: **yes**. Reinstall the agent from the server page
  after updating the panel to install agent `0.6.1` and the updated systemd
  unit.
- Database migration required: **no**.
- Update the panel with `docker compose pull && docker compose up -d`.

## [0.5.6] - 2026-08-13

### Panel

- Added a Notifications page backed by live monitoring signals, with
  actionable node and peer links, per-account read state and an unread marker
  in the dashboard header.
- Linked the Monitoring health summary to the concrete notifications that
  require attention.
- Widened the Activity result column for more consistent status badges.

### Agent

- No functional changes.

### Upgrade notes

- Agent update required: **no**.
- Database migration required: **no**.
- Update the panel with `docker compose pull && docker compose up -d`.

## [0.5.5] - 2026-08-13

### Panel

- Reinstallation commands now preserve the actual WireGuard interface,
  listen port and inferred /24 subnet of an existing or imported node instead
  of always falling back to `peerblade0` and `10.44.0.1/24`.
- Traffic charts now include an adaptive vertical byte scale and horizontal
  grid lines for easier comparison across time periods.

### Agent

- No functional changes.

### Upgrade notes

- Agent update required: **no**.
- Database migration required: **no**.
- Update the panel with `docker compose pull && docker compose up -d`.

## [0.5.4] - 2026-08-13

### Website

- Strengthened the hero positioning around PeerBlade as a self-hosted
  WireGuard control plane in both English and Russian.
- Added documentation, deployment, FAQ and GitHub navigation, plus a compact
  FAQ section with structured data on the landing page.
- Added a visible path to the step-by-step guide and public deployment bundle
  alongside the one-command installation flow.
- Added sticky navigation with active-section highlighting, scroll-aware
  visibility and a button for returning to the top of the page.
- Added feature-card reveal motion and optimized continuous animations,
  scrolling work and mobile rendering for lower-powered devices.
- Reworked the fictional interface previews for narrow screens so peer data is
  presented as readable cards rather than clipped desktop tables.

### Panel

- Rebuilt the mobile peer registry as responsive cards without horizontal
  scrolling.
- Improved mobile node cards, headings, actions and spacing on the Servers
  page.

### Agent

- No functional changes.

### Upgrade notes

- Agent update required: **no**.
- Database migration required: **no**.
- Update the panel with `docker compose pull && docker compose up -d`.

## [0.5.3] - 2026-08-12

### Panel

- Refined the landing page and control plane palette with deeper mint accents,
  warmer surfaces and stronger contrast.
- Unified borders across floating dashboard surfaces and the interface previews
  shown on the landing page.
- Simplified the landing navigation background and polished server cards,
  outlined actions, language controls and the status refresh button.

### Agent

- No functional changes.

### Upgrade notes

- Agent update required: **no**.
- Database migration required: **no**.
- Update the panel with `docker compose pull && docker compose up -d`.

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
