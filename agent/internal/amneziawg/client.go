package amneziawg

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type commandRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(name string, args ...string) ([]byte, error) {
	output, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

// Client adapts the official awg control tool to the subset of wgctrl used by
// PeerBlade. AmneziaWG retains WireGuard's keys, peers and traffic counters.
type Client struct {
	command string
	runner  commandRunner
}

func NewClient() *Client {
	return &Client{command: "awg", runner: execRunner{}}
}

func (c *Client) Device(name string) (*wgtypes.Device, error) {
	output, err := c.runner.Run(c.command, "show", name, "dump")
	if err != nil {
		return nil, fmt.Errorf("read AmneziaWG interface %q: %w", name, err)
	}
	return parseDump(name, string(output))
}

func (c *Client) ConfigureDevice(name string, config wgtypes.Config) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("AmneziaWG interface name is required")
	}
	for _, peer := range config.Peers {
		args := []string{"set", name, "peer", peer.PublicKey.String()}
		if peer.Remove {
			args = append(args, "remove")
			if _, err := c.runner.Run(c.command, args...); err != nil {
				return fmt.Errorf("remove AmneziaWG peer: %w", err)
			}
			continue
		}

		var temporaryKey string
		if peer.PresharedKey != nil {
			file, err := os.CreateTemp("", "peerblade-awg-psk-*")
			if err != nil {
				return fmt.Errorf("create temporary preshared key file: %w", err)
			}
			temporaryKey = file.Name()
			if err := file.Chmod(0o600); err != nil {
				file.Close()
				os.Remove(temporaryKey)
				return fmt.Errorf("protect temporary preshared key file: %w", err)
			}
			if _, err := file.WriteString(peer.PresharedKey.String() + "\n"); err != nil {
				file.Close()
				os.Remove(temporaryKey)
				return fmt.Errorf("write temporary preshared key file: %w", err)
			}
			if err := file.Close(); err != nil {
				os.Remove(temporaryKey)
				return fmt.Errorf("close temporary preshared key file: %w", err)
			}
			defer os.Remove(temporaryKey)
			args = append(args, "preshared-key", temporaryKey)
		}
		if peer.ReplaceAllowedIPs {
			allowedIPs := make([]string, 0, len(peer.AllowedIPs))
			for _, allowedIP := range peer.AllowedIPs {
				allowedIPs = append(allowedIPs, allowedIP.String())
			}
			args = append(args, "allowed-ips", strings.Join(allowedIPs, ","))
		}
		if peer.PersistentKeepaliveInterval != nil {
			seconds := int64(*peer.PersistentKeepaliveInterval / time.Second)
			args = append(args, "persistent-keepalive", strconv.FormatInt(seconds, 10))
		}
		if _, err := c.runner.Run(c.command, args...); err != nil {
			return fmt.Errorf("configure AmneziaWG peer: %w", err)
		}
	}
	return nil
}

func parseDump(name string, raw string) (*wgtypes.Device, error) {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, errors.New("AmneziaWG returned an empty interface dump")
	}
	interfaceFields := strings.Split(lines[0], "\t")
	if len(interfaceFields) < 4 {
		return nil, errors.New("AmneziaWG interface dump has an invalid header")
	}
	privateKey, err := parseOptionalKey(interfaceFields[0])
	if err != nil {
		return nil, fmt.Errorf("parse AmneziaWG private key: %w", err)
	}
	publicKey, err := parseOptionalKey(interfaceFields[1])
	if err != nil {
		return nil, fmt.Errorf("parse AmneziaWG public key: %w", err)
	}
	listenPort, err := strconv.Atoi(interfaceFields[2])
	if err != nil {
		return nil, fmt.Errorf("parse AmneziaWG listen port: %w", err)
	}
	firewallMark, err := strconv.ParseUint(strings.TrimPrefix(interfaceFields[3], "0x"), 16, 32)
	if err != nil {
		firewallMark, err = strconv.ParseUint(interfaceFields[3], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parse AmneziaWG firewall mark: %w", err)
		}
	}
	device := &wgtypes.Device{
		Name: name, Type: wgtypes.Userspace, PrivateKey: privateKey,
		PublicKey: publicKey, ListenPort: listenPort, FirewallMark: int(firewallMark),
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		peer, err := parsePeer(strings.Split(line, "\t"))
		if err != nil {
			return nil, err
		}
		device.Peers = append(device.Peers, peer)
	}
	return device, nil
}

func parsePeer(fields []string) (wgtypes.Peer, error) {
	if len(fields) < 8 {
		return wgtypes.Peer{}, errors.New("AmneziaWG peer dump has too few fields")
	}
	publicKey, err := wgtypes.ParseKey(fields[0])
	if err != nil {
		return wgtypes.Peer{}, fmt.Errorf("parse AmneziaWG peer public key: %w", err)
	}
	presharedKey, err := parseOptionalKey(fields[1])
	if err != nil {
		return wgtypes.Peer{}, fmt.Errorf("parse AmneziaWG preshared key: %w", err)
	}
	peer := wgtypes.Peer{PublicKey: publicKey, PresharedKey: presharedKey}
	if fields[2] != "(none)" {
		peer.Endpoint, err = net.ResolveUDPAddr("udp", fields[2])
		if err != nil {
			return wgtypes.Peer{}, fmt.Errorf("parse AmneziaWG endpoint: %w", err)
		}
	}
	if fields[3] != "(none)" && fields[3] != "" {
		for _, value := range strings.Split(fields[3], ",") {
			_, network, parseErr := net.ParseCIDR(value)
			if parseErr != nil {
				return wgtypes.Peer{}, fmt.Errorf("parse AmneziaWG allowed IP: %w", parseErr)
			}
			peer.AllowedIPs = append(peer.AllowedIPs, *network)
		}
	}
	handshake, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil {
		return wgtypes.Peer{}, fmt.Errorf("parse AmneziaWG handshake: %w", err)
	}
	if handshake > 0 {
		peer.LastHandshakeTime = time.Unix(handshake, 0).UTC()
	}
	peer.ReceiveBytes, err = strconv.ParseInt(fields[5], 10, 64)
	if err != nil {
		return wgtypes.Peer{}, fmt.Errorf("parse AmneziaWG received bytes: %w", err)
	}
	peer.TransmitBytes, err = strconv.ParseInt(fields[6], 10, 64)
	if err != nil {
		return wgtypes.Peer{}, fmt.Errorf("parse AmneziaWG transmitted bytes: %w", err)
	}
	peer.PersistentKeepaliveInterval, err = parseKeepalive(fields[7])
	if err != nil {
		return wgtypes.Peer{}, err
	}
	return peer, nil
}

func parseKeepalive(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "off" {
		return 0, nil
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse AmneziaWG keepalive: %w", err)
	}
	if seconds < 0 {
		return 0, errors.New("parse AmneziaWG keepalive: value cannot be negative")
	}
	return time.Duration(seconds) * time.Second, nil
}

func parseOptionalKey(value string) (wgtypes.Key, error) {
	if value == "(none)" || value == "" {
		return wgtypes.Key{}, nil
	}
	return wgtypes.ParseKey(value)
}
