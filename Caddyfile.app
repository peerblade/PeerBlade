{
	admin off
}

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
		reverse_proxy peerblade-web:3000
	}

	@agent path /api/agent/*
	handle @agent {
		reverse_proxy peerblade-api:4000
	}

	@health path /api/health
	handle @health {
		reverse_proxy peerblade-api:4000
	}

	handle /api/* {
		reverse_proxy peerblade-api:4000
	}

	handle {
		reverse_proxy peerblade-web:3000
	}
}

{$PEERBLADE_MARKETING_DOMAIN} {
	encode zstd gzip

	header {
		Strict-Transport-Security "max-age=31536000; includeSubDomains"
		X-Content-Type-Options "nosniff"
		X-Frame-Options "DENY"
		Referrer-Policy "same-origin"
		-Server
	}

	@controlPlane path /api/* /install-agent.sh /reconnect-agent.sh /uninstall-agent.sh /downloads/*
	@installationReceipt path /api/installations
	handle @installationReceipt {
		reverse_proxy peerblade-api:4000
	}

	handle @controlPlane {
		redir https://{$PEERBLADE_DOMAIN}{uri} 308
	}

	@panelRoutes path /login /setup /servers* /peers* /monitoring* /activity* /settings* /status*
	redir @panelRoutes https://{$PEERBLADE_DOMAIN}{uri} 308

	handle {
		reverse_proxy peerblade-web:3000
	}
}

{$PEERBLADE_DEV_DOMAIN} {
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
		reverse_proxy peerblade-dev-web:3000
	}

	@agent path /api/agent/*
	handle @agent {
		reverse_proxy peerblade-dev-api:4000
	}

	@health path /api/health
	handle @health {
		reverse_proxy peerblade-dev-api:4000
	}

	handle /api/* {
		basic_auth {
			{$PEERBLADE_DEV_ADMIN_USER} {$PEERBLADE_DEV_ADMIN_PASSWORD_HASH}
		}

		reverse_proxy peerblade-dev-api:4000
	}

	handle {
		basic_auth {
			{$PEERBLADE_DEV_ADMIN_USER} {$PEERBLADE_DEV_ADMIN_PASSWORD_HASH}
		}

		reverse_proxy peerblade-dev-web:3000
	}
}
