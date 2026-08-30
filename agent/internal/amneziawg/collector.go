package amneziawg

import (
	"fmt"
	"sort"

	"github.com/peerblade/PeerBlade/agent/internal/wireguard"
)

type snapshotSource interface {
	Collect() (wireguard.Snapshot, error)
}

// Collector appends the selected AWG interface to the normalised snapshot.
type Collector struct {
	source        snapshotSource
	client        *Client
	interfaceName string
}

func NewCollector(source snapshotSource, client *Client, interfaceName string) *Collector {
	return &Collector{source: source, client: client, interfaceName: interfaceName}
}

func (c *Collector) Collect() (wireguard.Snapshot, error) {
	snapshot, err := c.source.Collect()
	if err != nil {
		return wireguard.Snapshot{}, err
	}
	device, err := c.client.Device(c.interfaceName)
	if err != nil {
		return wireguard.Snapshot{}, fmt.Errorf("collect AmneziaWG interface: %w", err)
	}
	mapped := wireguard.MapDevice(device, "amneziawg")
	mapped.Type = "AmneziaWG"
	replaced := false
	for index := range snapshot.Devices {
		if snapshot.Devices[index].Name == c.interfaceName {
			snapshot.Devices[index] = mapped
			replaced = true
			break
		}
	}
	if !replaced {
		snapshot.Devices = append(snapshot.Devices, mapped)
	}
	sort.Slice(snapshot.Devices, func(i, j int) bool {
		return snapshot.Devices[i].Name < snapshot.Devices[j].Name
	})
	return snapshot, nil
}
