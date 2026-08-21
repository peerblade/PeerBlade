package nativewg

import (
	"strings"
	"testing"

	"github.com/peerblade/PeerBlade/agent/internal/wireguard"
)

type staticSnapshotSource struct {
	snapshot wireguard.Snapshot
}

func (s staticSnapshotSource) Collect() (wireguard.Snapshot, error) {
	return s.snapshot, nil
}

func TestManagedCollectorDecoratesNativePeer(t *testing.T) {
	manager, _, _ := newTestManager(t)
	created, err := manager.CreatePeer("Phone")
	if err != nil {
		t.Fatal(err)
	}
	source := staticSnapshotSource{snapshot: wireguard.Snapshot{
		SchemaVersion: 1,
		Devices: []wireguard.DeviceSnapshot{{
			Name: "peerblade0",
			Peers: []wireguard.PeerSnapshot{{
				PublicKey:     created.PublicKey,
				AllowedIPs:    []string{"10.44.0.2/32"},
				ReceiveBytes:  "12",
				TransmitBytes: "34",
			}},
		}},
	}}

	snapshot, err := NewManagedCollector(source, manager).Collect()
	if err != nil {
		t.Fatal(err)
	}
	peer := snapshot.Devices[0].Peers[0]
	if peer.ManagedBy != "peerblade_native" || peer.ManagedName != "Phone" {
		t.Fatalf("managed metadata = %+v", peer)
	}
	if peer.Enabled == nil || !*peer.Enabled {
		t.Fatalf("enabled metadata = %v", peer.Enabled)
	}
	if got := strings.Join(peer.ConfiguredAllowedIPs, ","); got != "10.44.0.2/32" {
		t.Fatalf("configured AllowedIPs = %q", got)
	}
}

func TestManagedCollectorRejectsMissingNativeInterface(t *testing.T) {
	manager, _, _ := newTestManager(t)
	_, err := NewManagedCollector(
		staticSnapshotSource{snapshot: wireguard.Snapshot{SchemaVersion: 1}},
		manager,
	).Collect()
	if err == nil || !strings.Contains(err.Error(), "managed interface peerblade0") {
		t.Fatalf("Collect() error = %v", err)
	}
}
