package controlplane

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeHeartbeatClient struct {
	called chan string
	err    error
}

func (f fakeHeartbeatClient) Heartbeat(_ context.Context, version string) (AgentResponse, error) {
	if f.called != nil {
		f.called <- version
	}
	return AgentResponse{}, f.err
}

func TestRunHeartbeatsStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	called := make(chan string, 1)
	done := make(chan error, 1)

	go func() {
		done <- RunHeartbeats(
			ctx,
			fakeHeartbeatClient{called: called},
			"0.1.0",
			time.Millisecond,
		)
	}()

	select {
	case version := <-called:
		if version != "0.1.0" {
			t.Fatalf("version = %q", version)
		}
		cancel()
	case <-time.After(time.Second):
		t.Fatal("heartbeat was not sent")
	}

	if err := <-done; err != nil {
		t.Fatalf("RunHeartbeats() error = %v", err)
	}
}

func TestRunHeartbeatsWrapsRequestError(t *testing.T) {
	err := RunHeartbeats(
		context.Background(),
		fakeHeartbeatClient{err: errors.New("connection refused")},
		"0.1.0",
		time.Millisecond,
	)

	if err == nil || !strings.Contains(err.Error(), "send heartbeat: connection refused") {
		t.Fatalf("RunHeartbeats() error = %v", err)
	}
}

func TestRunHeartbeatsRejectsInvalidInterval(t *testing.T) {
	err := RunHeartbeats(
		context.Background(),
		fakeHeartbeatClient{},
		"0.1.0",
		0,
	)

	if err == nil {
		t.Fatal("RunHeartbeats() accepted zero interval")
	}
}
