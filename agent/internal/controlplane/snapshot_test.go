package controlplane

import (
	"context"
	"testing"
	"time"

	"github.com/peerblade/PeerBlade/agent/internal/wireguard"
)

type fakeSnapshotCollector struct {
	snapshot wireguard.Snapshot
	calls    int
}

func (f *fakeSnapshotCollector) Collect() (wireguard.Snapshot, error) {
	f.calls++
	return f.snapshot, nil
}

type fakeSnapshotClient struct {
	snapshots []wireguard.Snapshot
	uploaded  chan struct{}
}

func (f *fakeSnapshotClient) Snapshot(
	_ context.Context,
	snapshot wireguard.Snapshot,
) (AgentResponse, error) {
	f.snapshots = append(f.snapshots, snapshot)
	f.uploaded <- struct{}{}
	return AgentResponse{}, nil
}

func TestRunSnapshotsUploadsImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &fakeSnapshotClient{uploaded: make(chan struct{}, 1)}
	collector := &fakeSnapshotCollector{snapshot: wireguard.Snapshot{SchemaVersion: 1}}

	done := make(chan error, 1)
	go func() {
		done <- RunSnapshots(ctx, client, collector, time.Hour)
	}()

	select {
	case <-client.uploaded:
	case <-time.After(time.Second):
		t.Fatal("snapshot was not uploaded immediately")
	}
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("RunSnapshots() error = %v", err)
	}
	if collector.calls != 1 || len(client.snapshots) != 1 {
		t.Fatalf("collect calls = %d, snapshots = %d", collector.calls, len(client.snapshots))
	}
}
