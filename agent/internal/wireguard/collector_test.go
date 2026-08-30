package wireguard

import (
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type fakeClient struct {
	devices []*wgtypes.Device
	err     error
}

func (f fakeClient) Devices() ([]*wgtypes.Device, error) {
	return f.devices, f.err
}

func TestCollectorCollectsSafeSnapshot(t *testing.T) {
	collectedAt := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.FixedZone("YEKT", 5*60*60))
	handshakeAt := time.Date(2026, time.August, 1, 6, 30, 0, 0, time.UTC)
	privateKey := testKey(1)
	presharedKey := testKey(2)
	devicePublicKey := testKey(3)
	peerPublicKey := testKey(4)

	collector := NewCollector(fakeClient{devices: []*wgtypes.Device{
		{
			Name:         "wg1",
			Type:         wgtypes.LinuxKernel,
			PrivateKey:   privateKey,
			PublicKey:    devicePublicKey,
			ListenPort:   51821,
			FirewallMark: 42,
			Peers: []wgtypes.Peer{
				{
					PublicKey:                   peerPublicKey,
					PresharedKey:                presharedKey,
					Endpoint:                    &net.UDPAddr{IP: net.ParseIP("203.0.113.10"), Port: 51820},
					PersistentKeepaliveInterval: 25 * time.Second,
					LastHandshakeTime:           handshakeAt,
					ReceiveBytes:                1024,
					TransmitBytes:               2048,
					AllowedIPs: []net.IPNet{
						mustCIDR(t, "10.20.0.3/32"),
						mustCIDR(t, "10.20.0.2/32"),
					},
				},
			},
		},
	}})
	collector.now = func() time.Time { return collectedAt }

	snapshot, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if snapshot.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want 1", snapshot.SchemaVersion)
	}
	if got, want := snapshot.CollectedAt, collectedAt.UTC(); !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("CollectedAt = %v, want %v in UTC", got, want)
	}
	if len(snapshot.Devices) != 1 {
		t.Fatalf("len(Devices) = %d, want 1", len(snapshot.Devices))
	}

	device := snapshot.Devices[0]
	if device.Name != "wg1" || device.Transport != "wireguard" || device.ListenPort != 51821 || device.PeerCount != 1 {
		t.Fatalf("unexpected device snapshot: %+v", device)
	}

	peer := device.Peers[0]
	if peer.Endpoint == nil || *peer.Endpoint != "203.0.113.10:51820" {
		t.Fatalf("Endpoint = %v, want 203.0.113.10:51820", peer.Endpoint)
	}
	if got, want := strings.Join(peer.AllowedIPs, ","), "10.20.0.2/32,10.20.0.3/32"; got != want {
		t.Fatalf("AllowedIPs = %q, want %q", got, want)
	}
	if peer.LastHandshakeAt == nil || !peer.LastHandshakeAt.Equal(handshakeAt) {
		t.Fatalf("LastHandshakeAt = %v, want %v", peer.LastHandshakeAt, handshakeAt)
	}
	if peer.ReceiveBytes != "1024" || peer.TransmitBytes != "2048" || peer.PersistentKeepaliveSeconds != 25 {
		t.Fatalf("unexpected peer counters: %+v", peer)
	}

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	jsonOutput := string(encoded)
	if strings.Contains(jsonOutput, privateKey.String()) {
		t.Fatal("snapshot contains device private key")
	}
	if strings.Contains(jsonOutput, presharedKey.String()) {
		t.Fatal("snapshot contains peer preshared key")
	}
}

func TestCollectorReturnsEmptyDeviceList(t *testing.T) {
	collector := NewCollector(fakeClient{})

	snapshot, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if snapshot.Devices == nil || len(snapshot.Devices) != 0 {
		t.Fatalf("Devices = %#v, want non-nil empty slice", snapshot.Devices)
	}
}

func TestCollectorWrapsClientError(t *testing.T) {
	collector := NewCollector(fakeClient{err: errors.New("permission denied")})

	_, err := collector.Collect()
	if err == nil || err.Error() != "read WireGuard devices: permission denied" {
		t.Fatalf("Collect() error = %v", err)
	}
}

func TestCollectorSkipsNilDevicesAndSortsByName(t *testing.T) {
	collector := NewCollector(fakeClient{devices: []*wgtypes.Device{
		{Name: "wg2"},
		nil,
		{Name: "wg0"},
	}})

	snapshot, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if got, want := snapshot.Devices[0].Name, "wg0"; got != want {
		t.Fatalf("first device = %q, want %q", got, want)
	}
}

func testKey(value byte) wgtypes.Key {
	var key wgtypes.Key
	for i := range key {
		key[i] = value
	}
	return key
}

func mustCIDR(t *testing.T, value string) net.IPNet {
	t.Helper()
	_, network, err := net.ParseCIDR(value)
	if err != nil {
		t.Fatalf("net.ParseCIDR(%q) error = %v", value, err)
	}
	return *network
}
