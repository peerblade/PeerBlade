package main

import (
	"encoding/json"
	"errors"

	"github.com/peerblade/PeerBlade/agent/internal/controlplane"
	"github.com/peerblade/PeerBlade/agent/internal/nativewg"
)

type nativeCommandExecutor struct {
	manager *nativewg.Manager
}

func (e *nativeCommandExecutor) Execute(command *controlplane.AgentCommand) (string, error) {
	switch command.Type {
	case "create_peer":
		created, err := e.manager.CreatePeer(command.Name)
		if err != nil {
			return "", err
		}
		payload, err := json.Marshal(created)
		if err != nil {
			return "", err
		}
		return string(payload), nil
	case "delete_peer":
		return "", e.manager.DeletePeer(command.PublicKey)
	case "get_peer_configuration":
		return e.manager.Configuration(command.PublicKey)
	case "disable_peer":
		return "", e.manager.SetEnabled(command.PublicKey, false)
	case "enable_peer":
		return "", e.manager.SetEnabled(command.PublicKey, true)
	default:
		return "", errors.New("unsupported native peer command")
	}
}
