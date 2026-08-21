package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"

	"github.com/peerblade/PeerBlade/agent/internal/controlplane"
	"github.com/peerblade/PeerBlade/agent/internal/nativewg"
	"github.com/peerblade/PeerBlade/agent/internal/wireguard"
)

var agentVersion = "0.6.1"

const usage = "usage: peerblade-agent [snapshot|register|run|version|import-wg-easy]"

type agentConfig struct {
	apiURL             string
	serverID           string
	token              string
	heartbeatInterval  time.Duration
	snapshotInterval   time.Duration
	commandInterval    time.Duration
	managedInterface   string
	managedEndpoint    string
	managedAddressCIDR string
	managedDNS         []string
	managedAllowedIPs  []string
	stateDirectory     string
}

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "peerblade agent: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	command := "snapshot"
	if len(args) > 0 {
		command = args[0]
	}
	if command == "import-wg-easy" {
		return runWGEasyImport(args[1:], output)
	}
	if len(args) > 1 {
		return errors.New(usage)
	}

	switch command {
	case "version":
		_, err := fmt.Fprintln(output, agentVersion)
		return err
	case "snapshot":
		return writeSnapshot(output)
	case "register", "run":
		config, err := loadAgentConfig()
		if err != nil {
			return err
		}

		client, err := controlplane.NewClient(
			config.apiURL,
			config.token,
			&http.Client{Timeout: 10 * time.Second},
		)
		if err != nil {
			return err
		}

		wireGuardClient, err := wgctrl.New()
		if err != nil {
			return fmt.Errorf("open WireGuard control client: %w", err)
		}
		defer wireGuardClient.Close()
		wireGuardControlClient, err := wgctrl.New()
		if err != nil {
			return fmt.Errorf("open WireGuard control client: %w", err)
		}
		defer wireGuardControlClient.Close()

		collector := interface {
			Collect() (wireguard.Snapshot, error)
		}(
			wireguard.NewCollector(wireGuardClient),
		)
		executor := interface {
			Execute(*controlplane.AgentCommand) (string, error)
		}(controlplane.NewPeerCommandExecutor(wireguard.NewController(wireGuardControlClient)))

		if config.managedInterface != "" {
			store, err := nativewg.NewStore(config.stateDirectory)
			if err != nil {
				return err
			}
			manager, err := nativewg.NewManager(wireGuardControlClient, store, nativewg.Config{
				InterfaceName:    config.managedInterface,
				Endpoint:         config.managedEndpoint,
				AddressCIDR:      config.managedAddressCIDR,
				DNS:              config.managedDNS,
				ClientAllowedIPs: config.managedAllowedIPs,
				KeepaliveSeconds: 25,
			})
			if err != nil {
				return err
			}
			if err := manager.Reconcile(); err != nil {
				return fmt.Errorf("reconcile managed peers: %w", err)
			}
			collector = nativewg.NewManagedCollector(collector, manager)
			executor = &nativeCommandExecutor{manager: manager}
		}

		capabilities := []string{"wireguard_snapshot", "peer_allowed_ips"}
		if config.managedInterface != "" {
			capabilities = append(capabilities, "native_peer_management")
		}
		agent, err := client.Register(ctx, config.serverID, agentVersion, capabilities)
		if err != nil {
			return fmt.Errorf("register with Control Plane: %w", err)
		}
		if err := writeJSON(output, agent); err != nil {
			return err
		}
		if command == "register" {
			return nil
		}

		return controlplane.RunAgent(
			ctx,
			client,
			collector,
			executor,
			agentVersion,
			config.heartbeatInterval,
			config.snapshotInterval,
			config.commandInterval,
		)
	default:
		return errors.New(usage)
	}
}

func runWGEasyImport(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("import-wg-easy", flag.ContinueOnError)
	flags.SetOutput(output)
	source := flags.String("source", "", "path to the wg-easy wg0.json file")
	stateDirectory := flags.String("state-directory", os.Getenv("PEERBLADE_STATE_DIRECTORY"), "PeerBlade agent state directory")
	write := flags.Bool("write", false, "write the validated peers to PeerBlade state")
	force := flags.Bool("force", false, "replace non-empty PeerBlade state; requires --write")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*source) == "" {
		return errors.New("import-wg-easy requires --source; omit --write for a dry-run")
	}
	if *force && !*write {
		return errors.New("--force requires --write")
	}

	state, summary, err := nativewg.LoadWGEasyState(strings.TrimSpace(*source))
	if err != nil {
		return err
	}
	mode := "dry-run"
	if *write {
		if strings.TrimSpace(*stateDirectory) == "" {
			return errors.New("--state-directory is required with --write")
		}
		store, err := nativewg.NewStore(strings.TrimSpace(*stateDirectory))
		if err != nil {
			return err
		}
		existing, err := store.Load()
		if err != nil {
			return err
		}
		if len(existing.Peers) > 0 && !*force {
			return errors.New("PeerBlade state is not empty; back it up and pass --force to replace it")
		}
		if err := store.Save(state); err != nil {
			return err
		}
		mode = "write"
	}

	return writeJSON(output, struct {
		Mode string `json:"mode"`
		nativewg.WGEasyImportSummary
	}{Mode: mode, WGEasyImportSummary: summary})
}

func writeSnapshot(output io.Writer) error {
	client, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("open WireGuard control client: %w", err)
	}
	defer client.Close()

	snapshot, err := wireguard.NewCollector(client).Collect()
	if err != nil {
		return err
	}

	return writeJSON(output, snapshot)
}

func writeJSON(output io.Writer, value interface{}) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}

	return nil
}

func loadAgentConfig() (agentConfig, error) {
	config := agentConfig{
		apiURL:             strings.TrimSpace(os.Getenv("PEERBLADE_API_URL")),
		serverID:           strings.TrimSpace(os.Getenv("PEERBLADE_SERVER_ID")),
		token:              strings.TrimSpace(os.Getenv("PEERBLADE_AGENT_TOKEN")),
		heartbeatInterval:  30 * time.Second,
		snapshotInterval:   60 * time.Second,
		commandInterval:    5 * time.Second,
		managedInterface:   strings.TrimSpace(os.Getenv("PEERBLADE_MANAGED_INTERFACE")),
		managedEndpoint:    strings.TrimSpace(os.Getenv("PEERBLADE_MANAGED_ENDPOINT")),
		managedAddressCIDR: strings.TrimSpace(os.Getenv("PEERBLADE_MANAGED_ADDRESS_CIDR")),
		managedDNS:         splitCSV(os.Getenv("PEERBLADE_MANAGED_DNS")),
		managedAllowedIPs:  splitCSV(os.Getenv("PEERBLADE_MANAGED_ALLOWED_IPS")),
		stateDirectory:     strings.TrimSpace(os.Getenv("PEERBLADE_STATE_DIRECTORY")),
	}
	if config.managedInterface != "" {
		for name, value := range map[string]string{
			"PEERBLADE_MANAGED_ENDPOINT":     config.managedEndpoint,
			"PEERBLADE_MANAGED_ADDRESS_CIDR": config.managedAddressCIDR,
			"PEERBLADE_STATE_DIRECTORY":      config.stateDirectory,
		} {
			if value == "" {
				return agentConfig{}, fmt.Errorf("%s is required for native peer management", name)
			}
		}
		if len(config.managedAllowedIPs) == 0 {
			config.managedAllowedIPs = []string{"0.0.0.0/0"}
		}
	}

	for name, value := range map[string]string{
		"PEERBLADE_API_URL":     config.apiURL,
		"PEERBLADE_SERVER_ID":   config.serverID,
		"PEERBLADE_AGENT_TOKEN": config.token,
	} {
		if value == "" {
			return agentConfig{}, fmt.Errorf("%s is required", name)
		}
	}

	if raw := strings.TrimSpace(os.Getenv("PEERBLADE_HEARTBEAT_INTERVAL")); raw != "" {
		interval, err := time.ParseDuration(raw)
		if err != nil {
			return agentConfig{}, fmt.Errorf("parse PEERBLADE_HEARTBEAT_INTERVAL: %w", err)
		}
		if interval < 5*time.Second {
			return agentConfig{}, errors.New("PEERBLADE_HEARTBEAT_INTERVAL must be at least 5s")
		}
		config.heartbeatInterval = interval
	}

	if raw := strings.TrimSpace(os.Getenv("PEERBLADE_SNAPSHOT_INTERVAL")); raw != "" {
		interval, err := time.ParseDuration(raw)
		if err != nil {
			return agentConfig{}, fmt.Errorf("parse PEERBLADE_SNAPSHOT_INTERVAL: %w", err)
		}
		if interval < 10*time.Second {
			return agentConfig{}, errors.New("PEERBLADE_SNAPSHOT_INTERVAL must be at least 10s")
		}
		config.snapshotInterval = interval
	}

	if raw := strings.TrimSpace(os.Getenv("PEERBLADE_COMMAND_INTERVAL")); raw != "" {
		interval, err := time.ParseDuration(raw)
		if err != nil {
			return agentConfig{}, fmt.Errorf("parse PEERBLADE_COMMAND_INTERVAL: %w", err)
		}
		if interval < time.Second {
			return agentConfig{}, errors.New("PEERBLADE_COMMAND_INTERVAL must be at least 1s")
		}
		config.commandInterval = interval
	}

	return config, nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if normalized := strings.TrimSpace(part); normalized != "" {
			result = append(result, normalized)
		}
	}
	return result
}
