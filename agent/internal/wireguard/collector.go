package wireguard

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const schemaVersion = 1

type deviceClient interface {
	Devices() ([]*wgtypes.Device, error)
}

type Collector struct {
	client deviceClient
	now    func() time.Time
}

type Snapshot struct {
	SchemaVersion int              `json:"schemaVersion"`
	CollectedAt   time.Time        `json:"collectedAt"`
	Devices       []DeviceSnapshot `json:"devices"`
}

type DeviceSnapshot struct {
	Name         string         `json:"name"`
	Transport    string         `json:"transport"`
	Type         string         `json:"type"`
	PublicKey    string         `json:"publicKey"`
	ListenPort   int            `json:"listenPort"`
	FirewallMark int            `json:"firewallMark"`
	PeerCount    int            `json:"peerCount"`
	Peers        []PeerSnapshot `json:"peers"`
}

type PeerSnapshot struct {
	PublicKey                  string     `json:"publicKey"`
	Endpoint                   *string    `json:"endpoint"`
	AllowedIPs                 []string   `json:"allowedIps"`
	LastHandshakeAt            *time.Time `json:"lastHandshakeAt"`
	ReceiveBytes               string     `json:"receiveBytes"`
	TransmitBytes              string     `json:"transmitBytes"`
	PersistentKeepaliveSeconds int64      `json:"persistentKeepaliveSeconds"`
	ManagedBy                  string     `json:"managedBy,omitempty"`
	ManagedName                string     `json:"managedName,omitempty"`
	ConfiguredAllowedIPs       []string   `json:"configuredAllowedIps,omitempty"`
	Enabled                    *bool      `json:"enabled,omitempty"`
}

func NewCollector(client deviceClient) *Collector {
	return &Collector{
		client: client,
		now:    time.Now,
	}
}

func (c *Collector) Collect() (Snapshot, error) {
	devices, err := c.client.Devices()
	if err != nil {
		return Snapshot{}, fmt.Errorf("read WireGuard devices: %w", err)
	}

	snapshot := Snapshot{
		SchemaVersion: schemaVersion,
		CollectedAt:   c.now().UTC(),
		Devices:       make([]DeviceSnapshot, 0, len(devices)),
	}

	for _, device := range devices {
		if device == nil {
			continue
		}

		snapshot.Devices = append(snapshot.Devices, MapDevice(device, "wireguard"))
	}

	sort.Slice(snapshot.Devices, func(i, j int) bool {
		return snapshot.Devices[i].Name < snapshot.Devices[j].Name
	})

	return snapshot, nil
}

func MapDevice(device *wgtypes.Device, transport string) DeviceSnapshot {
	peers := make([]PeerSnapshot, 0, len(device.Peers))

	for _, peer := range device.Peers {
		peers = append(peers, mapPeer(peer))
	}

	sort.Slice(peers, func(i, j int) bool {
		return peers[i].PublicKey < peers[j].PublicKey
	})

	return DeviceSnapshot{
		Name:         device.Name,
		Transport:    transport,
		Type:         device.Type.String(),
		PublicKey:    device.PublicKey.String(),
		ListenPort:   device.ListenPort,
		FirewallMark: device.FirewallMark,
		PeerCount:    len(peers),
		Peers:        peers,
	}
}

func mapPeer(peer wgtypes.Peer) PeerSnapshot {
	allowedIPs := make([]string, 0, len(peer.AllowedIPs))

	for _, allowedIP := range peer.AllowedIPs {
		allowedIPs = append(allowedIPs, allowedIP.String())
	}

	sort.Strings(allowedIPs)

	return PeerSnapshot{
		PublicKey:                  peer.PublicKey.String(),
		Endpoint:                   formatEndpoint(peer.Endpoint),
		AllowedIPs:                 allowedIPs,
		LastHandshakeAt:            optionalTime(peer.LastHandshakeTime),
		ReceiveBytes:               strconv.FormatInt(peer.ReceiveBytes, 10),
		TransmitBytes:              strconv.FormatInt(peer.TransmitBytes, 10),
		PersistentKeepaliveSeconds: int64(peer.PersistentKeepaliveInterval / time.Second),
	}
}

func formatEndpoint(endpoint *net.UDPAddr) *string {
	if endpoint == nil {
		return nil
	}

	value := endpoint.String()
	return &value
}

func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}

	utc := value.UTC()
	return &utc
}
