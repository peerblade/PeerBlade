package controlplane

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type commandClient interface {
	NextCommand(context.Context) (*AgentCommand, error)
	CompleteCommand(context.Context, string, AgentCommandResult) error
}

type commandExecutor interface {
	Execute(*AgentCommand) (string, error)
}

func RunCommands(
	ctx context.Context,
	client commandClient,
	executor commandExecutor,
	interval time.Duration,
) error {
	if interval <= 0 {
		return errors.New("command interval must be positive")
	}

	poll := func() error {
		command, err := client.NextCommand(ctx)
		if err != nil {
			return fmt.Errorf("claim command: %w", err)
		}
		if command == nil {
			return nil
		}

		result := AgentCommandResult{Status: "succeeded"}
		payload, executeError := executor.Execute(command)
		if executeError != nil {
			result.Status = "failed"
			result.Error = executeError.Error()
		} else {
			result.Payload = payload
		}

		if err := client.CompleteCommand(ctx, command.ID, result); err != nil {
			return fmt.Errorf("complete command: %w", err)
		}

		return nil
	}

	if err := poll(); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := poll(); err != nil {
				return err
			}
		}
	}
}
