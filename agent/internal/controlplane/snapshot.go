package controlplane

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/peerblade/PeerBlade/agent/internal/wireguard"
)

type snapshotClient interface {
	Snapshot(context.Context, wireguard.Snapshot) (AgentResponse, error)
}

type snapshotCollector interface {
	Collect() (wireguard.Snapshot, error)
}

type synchronizedCollector struct {
	inner snapshotCollector
	mu    sync.Mutex
}

func (c *synchronizedCollector) Collect() (wireguard.Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.inner.Collect()
}

func RunSnapshots(
	ctx context.Context,
	client snapshotClient,
	collector snapshotCollector,
	interval time.Duration,
) error {
	if interval <= 0 {
		return errors.New("snapshot interval must be positive")
	}

	upload := func() error {
		snapshot, err := collector.Collect()
		if err != nil {
			return fmt.Errorf("collect WireGuard snapshot: %w", err)
		}
		if _, err := client.Snapshot(ctx, snapshot); err != nil {
			return fmt.Errorf("upload WireGuard snapshot: %w", err)
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

func RunAgent(
	ctx context.Context,
	client *Client,
	collector snapshotCollector,
	executor commandExecutor,
	version string,
	heartbeatInterval time.Duration,
	snapshotInterval time.Duration,
	trafficInterval time.Duration,
	commandInterval time.Duration,
) error {
	const retryDelay = 5 * time.Second
	collector = &synchronizedCollector{inner: collector}

	workers := []struct {
		name string
		run  func(context.Context) error
	}{
		{
			name: "heartbeat",
			run: func(workerContext context.Context) error {
				return RunHeartbeats(workerContext, client, version, heartbeatInterval)
			},
		},
		{
			name: "snapshot",
			run: func(workerContext context.Context) error {
				return RunSnapshots(workerContext, client, collector, snapshotInterval)
			},
		},
		{
			name: "live traffic",
			run: func(workerContext context.Context) error {
				return RunTrafficReports(workerContext, client, collector, trafficInterval)
			},
		},
		{
			name: "command polling",
			run: func(workerContext context.Context) error {
				return RunCommands(workerContext, client, executor, commandInterval)
			},
		},
	}

	var workersGroup sync.WaitGroup
	workersGroup.Add(len(workers))

	for _, worker := range workers {
		go func() {
			defer workersGroup.Done()
			runWithRetry(ctx, worker.name, retryDelay, worker.run)
		}()
	}

	<-ctx.Done()
	workersGroup.Wait()
	return nil
}

func runWithRetry(
	ctx context.Context,
	name string,
	retryDelay time.Duration,
	run func(context.Context) error,
) {
	for {
		err := run(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("%s failed: %v; retrying in %s", name, err, retryDelay)
		} else {
			log.Printf("%s stopped unexpectedly; retrying in %s", name, retryDelay)
		}

		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
