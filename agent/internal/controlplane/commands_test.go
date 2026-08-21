package controlplane

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeCommandClient struct {
	command   *AgentCommand
	results   chan AgentCommandResult
	completed chan string
}

func (f *fakeCommandClient) NextCommand(context.Context) (*AgentCommand, error) {
	command := f.command
	f.command = nil
	return command, nil
}

func (f *fakeCommandClient) CompleteCommand(
	_ context.Context,
	id string,
	result AgentCommandResult,
) error {
	f.results <- result
	f.completed <- id
	return nil
}

type fakeCommandExecutor struct {
	interfaceName string
	publicKey     string
	allowedIPs    []string
	err           error
}

func (f *fakeCommandExecutor) Execute(command *AgentCommand) (string, error) {
	f.interfaceName = command.InterfaceName
	f.publicKey = command.PublicKey
	f.allowedIPs = command.AllowedIPs
	return "", f.err
}

func TestRunCommandsExecutesAndCompletesImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &fakeCommandClient{
		command: &AgentCommand{
			ID:            "command-id",
			Type:          "disable_peer",
			InterfaceName: "wg0",
			PublicKey:     "public-key",
			AllowedIPs:    []string{},
		},
		results:   make(chan AgentCommandResult, 1),
		completed: make(chan string, 1),
	}
	executor := &fakeCommandExecutor{}
	done := make(chan error, 1)

	go func() {
		done <- RunCommands(ctx, client, executor, time.Hour)
	}()

	select {
	case id := <-client.completed:
		if id != "command-id" {
			t.Fatalf("completed command = %q", id)
		}
		cancel()
	case <-time.After(time.Second):
		t.Fatal("command was not completed")
	}

	if err := <-done; err != nil {
		t.Fatalf("RunCommands() error = %v", err)
	}
	if result := <-client.results; result.Status != "succeeded" || result.Error != "" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if executor.interfaceName != "wg0" || executor.publicKey != "public-key" {
		t.Fatalf("unexpected executor call: %+v", executor)
	}
}

func TestRunCommandsReportsExecutorFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &fakeCommandClient{
		command: &AgentCommand{
			ID:            "command-id",
			Type:          "enable_peer",
			InterfaceName: "wg0",
			PublicKey:     "public-key",
			AllowedIPs:    []string{"10.8.0.2/32"},
		},
		results:   make(chan AgentCommandResult, 1),
		completed: make(chan string, 1),
	}
	executor := &fakeCommandExecutor{err: errors.New("permission denied")}
	done := make(chan error, 1)

	go func() {
		done <- RunCommands(ctx, client, executor, time.Hour)
	}()

	<-client.completed
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("RunCommands() error = %v", err)
	}
	if result := <-client.results; result.Status != "failed" || result.Error != "permission denied" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
