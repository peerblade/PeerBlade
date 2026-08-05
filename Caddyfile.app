{
	admin off
}

# PeerBlade panel. Caddy issues the TLS certificate automatically,
# so PEERBLADE_DOMAIN must already resolve to this host.
{$PEERBLADE_DOMAIN} {
	encode zstd gzip

	header {
		Strict-Transport-Security "max-age=31536000; includeSubDomains"
		X-Content-Type-Options "nosniff"
		X-Frame-Options "DENY"
		Referrer-Policy "same-origin"
		-Server
	}

	@panelRoot path /
	redir @panelRoot /servers 307

	@agentInstaller path /install-agent.sh /reconnect-agent.sh /uninstall-agent.sh /downloads/*
	handle @agentInstaller {
		reverse_proxy {$PEERBLADE_WEB_HOSTNAME:peerblade-web}:3000
	}

	@agent path /api/agent/*
	handle @agent {
		reverse_proxy {$PEERBLADE_API_HOSTNAME:peerblade-api}:4000
	}

	@health path /api/health
	handle @health {
		reverse_proxy {$PEERBLADE_API_HOSTNAME:peerblade-api}:4000
	}

	handle /api/* {
		reverse_proxy {$PEERBLADE_API_HOSTNAME:peerblade-api}:4000
	}

	handle {
		reverse_proxy {$PEERBLADE_WEB_HOSTNAME:peerblade-web}:3000
	}
}
