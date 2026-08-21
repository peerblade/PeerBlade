package wireguard

import (
	"fmt"
	"net"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type deviceConfigurator interface {
	ConfigureDevice(name string, config wgtypes.Config) error
}

type Controller struct {
	client deviceConfigurator
}

func NewController(client deviceConfigurator) *Controller {
	return &Controller{client: client}
}

func (c *Controller) SetPeerAllowedIPs(
	interfaceName string,
	publicKey string,
	allowedIPs []string,
) error {
	key, err := wgtypes.ParseKey(publicKey)
	if err != nil {
		return fmt.Errorf("parse peer public key: %w", err)
	}

	parsedAllowedIPs := make([]net.IPNet, 0, len(allowedIPs))
	for _, allowedIP := range allowedIPs {
		_, network, err := net.ParseCIDR(allowedIP)
		if err != nil {
			return fmt.Errorf("parse allowed IP %q: %w", allowedIP, err)
		}
		parsedAllowedIPs = append(parsedAllowedIPs, *network)
	}

	if err := c.client.ConfigureDevice(interfaceName, wgtypes.Config{
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:         key,
				ReplaceAllowedIPs: true,
				AllowedIPs:        parsedAllowedIPs,
			},
		},
	}); err != nil {
		return fmt.Errorf("configure WireGuard interface %q: %w", interfaceName, err)
	}

	return nil
}
