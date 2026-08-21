package controlplane

import "errors"

type peerController interface {
	SetPeerAllowedIPs(string, string, []string) error
}

type PeerCommandExecutor struct {
	controller peerController
}

func NewPeerCommandExecutor(controller peerController) *PeerCommandExecutor {
	return &PeerCommandExecutor{controller: controller}
}

func (e *PeerCommandExecutor) Execute(command *AgentCommand) (string, error) {
	if command.Type != "disable_peer" && command.Type != "enable_peer" {
		return "", errors.New("command requires native peer management")
	}
	return "", e.controller.SetPeerAllowedIPs(
		command.InterfaceName,
		command.PublicKey,
		command.AllowedIPs,
	)
}
