package controlplane

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/peerblade/PeerBlade/agent/internal/wireguard"
)

type TrafficSample struct {
	InterfaceName string `json:"interfaceName"`
	PublicKey     string `json:"publicKey"`
	ReceiveBytes  string `json:"receiveBytes"`
	TransmitBytes string `json:"transmitBytes"`
}

type TrafficReport struct {
	SchemaVersion int             `json:"schemaVersion"`
	CollectedAt   time.Time       `json:"collectedAt"`
	Peers         []TrafficSample `json:"peers"`
}

type trafficClient interface {
	Traffic(context.Context, TrafficReport) error
}

func trafficReport(snapshot wireguard.Snapshot) TrafficReport {
	report := TrafficReport{
		SchemaVersion: 1,
		CollectedAt:   snapshot.CollectedAt,
		Peers:         make([]TrafficSample, 0),
	}
	for _, device := range snapshot.Devices {
		for _, peer := range device.Peers {
			report.Peers = append(report.Peers, TrafficSample{
				InterfaceName: device.Name,
				PublicKey:     peer.PublicKey,
				ReceiveBytes:  peer.ReceiveBytes,
				TransmitBytes: peer.TransmitBytes,
			})
		}
	}
	return report
}

func RunTrafficReports(
	ctx context.Context,
	client trafficClient,
	collector snapshotCollector,
	interval time.Duration,
) error {
	if interval <= 0 {
		return errors.New("traffic interval must be positive")
	}

	upload := func() error {
		snapshot, err := collector.Collect()
		if err != nil {
			return fmt.Errorf("collect live WireGuard counters: %w", err)
		}
		if err := client.Traffic(ctx, trafficReport(snapshot)); err != nil {
			return fmt.Errorf("upload live WireGuard counters: %w", err)
		}
		return nil
	}

	if err := upload(); err != nil {
		return err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := upload(); err != nil {
				return err
			}
		}
	}
}
