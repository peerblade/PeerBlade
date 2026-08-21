package controlplane

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type heartbeatClient interface {
	Heartbeat(context.Context, string) (AgentResponse, error)
}

func RunHeartbeats(
	ctx context.Context,
	client heartbeatClient,
	version string,
	interval time.Duration,
) error {
	if interval <= 0 {
		return errors.New("heartbeat interval must be positive")
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := client.Heartbeat(ctx, version); err != nil {
				return fmt.Errorf("send heartbeat: %w", err)
			}
		}
	}
}
