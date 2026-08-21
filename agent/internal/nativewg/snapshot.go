package nativewg

import (
	"fmt"
	"sort"

	"github.com/peerblade/PeerBlade/agent/internal/wireguard"
)

type snapshotSource interface {
	Collect() (wireguard.Snapshot, error)
}

type ManagedCollector struct {
	source  snapshotSource
	manager *Manager
}

func NewManagedCollector(source snapshotSource, manager *Manager) *ManagedCollector {
	return &ManagedCollector{source: source, manager: manager}
}

func (c *ManagedCollector) Collect() (wireguard.Snapshot, error) {
	snapshot, err := c.source.Collect()
	if err != nil {
		return wireguard.Snapshot{}, err
	}
	managedPeers, err := c.manager.Peers()
	if err != nil {
		return wireguard.Snapshot{}, err
	}

	foundManagedInterface := false
	for deviceIndex := range snapshot.Devices {
		device := &snapshot.Devices[deviceIndex]
		if device.Name != c.manager.config.InterfaceName {
			continue
		}
		foundManagedInterface = true
		for _, managed := range managedPeers {
			peerIndex := -1
			for index := range device.Peers {
				if device.Peers[index].PublicKey == managed.PublicKey {
					peerIndex = index
					break
				}
			}
			if peerIndex < 0 {
				device.Peers = append(device.Peers, wireguard.PeerSnapshot{
					PublicKey: managed.PublicKey, AllowedIPs: []string{}, ReceiveBytes: "0", TransmitBytes: "0",
				})
				peerIndex = len(device.Peers) - 1
			}
			peer := &device.Peers[peerIndex]
			peer.ManagedBy = "peerblade_native"
			peer.ManagedName = managed.Name
			peer.ConfiguredAllowedIPs = append([]string(nil), managed.AllowedIPs...)
			peer.Enabled = &managed.Enabled
		}
		sort.Slice(device.Peers, func(i, j int) bool { return device.Peers[i].PublicKey < device.Peers[j].PublicKey })
		device.PeerCount = len(device.Peers)
	}
	if !foundManagedInterface {
		return wireguard.Snapshot{}, fmt.Errorf(
			"managed interface %s is missing from WireGuard snapshot",
			c.manager.config.InterfaceName,
		)
	}

	return snapshot, nil
}
