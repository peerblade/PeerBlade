package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/peerblade/PeerBlade/agent/internal/nativewg"
)

func TestRunRejectsUnknownCommand(t *testing.T) {
	err := run(context.Background(), []string{"unknown"}, &strings.Builder{})

	if err == nil || err.Error() != usage {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunPrintsVersionWithoutConfiguration(t *testing.T) {
	var output strings.Builder

	err := run(context.Background(), []string{"version"}, &output)

	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := strings.TrimSpace(output.String()), agentVersion; got != want {
		t.Fatalf("version = %q, want %q", got, want)
	}
}

func TestRunWGEasyImportRequiresSource(t *testing.T) {
	err := run(context.Background(), []string{"import-wg-easy"}, &strings.Builder{})

	if err == nil || !strings.Contains(err.Error(), "requires --source") {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunWGEasyImportRejectsForceDuringDryRun(t *testing.T) {
	err := run(
		context.Background(),
		[]string{"import-wg-easy", "--source", "/tmp/wg0.json", "--force"},
		&strings.Builder{},
	)

	if err == nil || !strings.Contains(err.Error(), "--force requires --write") {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunWGEasyImportDryRunAndWrite(t *testing.T) {
	serverPrivateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	clientPrivateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	presharedKey, err := wgtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	document := map[string]interface{}{
		"server": map[string]interface{}{
			"address":    "10.8.0.1",
			"privateKey": serverPrivateKey.String(),
			"publicKey":  serverPrivateKey.PublicKey().String(),
		},
		"clients": map[string]interface{}{
			"peer": map[string]interface{}{
				"name":         "Phone",
				"address":      "10.8.0.2",
				"privateKey":   clientPrivateKey.String(),
				"publicKey":    clientPrivateKey.PublicKey().String(),
				"preSharedKey": presharedKey.String(),
				"enabled":      true,
			},
		},
	}
	contents, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "wg0.json")
	if err := os.WriteFile(source, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	stateDirectory := filepath.Join(t.TempDir(), "state")

	var dryRunOutput strings.Builder
	err = run(
		context.Background(),
		[]string{"import-wg-easy", "--source", source},
		&dryRunOutput,
	)
	if err != nil || !strings.Contains(dryRunOutput.String(), `"mode": "dry-run"`) {
		t.Fatalf("dry-run error = %v, output = %s", err, dryRunOutput.String())
	}
	if _, err := os.Stat(filepath.Join(stateDirectory, "peers.json")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created state: %v", err)
	}

	var writeOutput strings.Builder
	err = run(
		context.Background(),
		[]string{
			"import-wg-easy", "--source", source,
			"--state-directory", stateDirectory, "--write",
		},
		&writeOutput,
	)
	if err != nil || !strings.Contains(writeOutput.String(), `"mode": "write"`) {
		t.Fatalf("write error = %v, output = %s", err, writeOutput.String())
	}
	store, err := nativewg.NewStore(stateDirectory)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil || len(state.Peers) != 1 || state.Peers[0].Name != "Phone" {
		t.Fatalf("unexpected imported state: %+v, error = %v", state, err)
	}
}

func TestLoadAgentConfig(t *testing.T) {
	t.Setenv("PEERBLADE_API_URL", " http://localhost:4000 ")
	t.Setenv("PEERBLADE_SERVER_ID", " server-id ")
	t.Setenv("PEERBLADE_AGENT_TOKEN", " pbl_token ")
	t.Setenv("PEERBLADE_HEARTBEAT_INTERVAL", "15s")
	t.Setenv("PEERBLADE_SNAPSHOT_INTERVAL", "45s")
	t.Setenv("PEERBLADE_COMMAND_INTERVAL", "3s")

	config, err := loadAgentConfig()
	if err != nil {
		t.Fatalf("loadAgentConfig() error = %v", err)
	}
	if config.apiURL != "http://localhost:4000" || config.serverID != "server-id" || config.token != "pbl_token" {
		t.Fatalf("unexpected config: %+v", config)
	}
	if config.heartbeatInterval != 15*time.Second {
		t.Fatalf("heartbeatInterval = %v", config.heartbeatInterval)
	}
	if config.snapshotInterval != 45*time.Second {
		t.Fatalf("snapshotInterval = %v", config.snapshotInterval)
	}
	if config.commandInterval != 3*time.Second {
		t.Fatalf("commandInterval = %v", config.commandInterval)
	}
}

func TestLoadAgentConfigRequiresSecrets(t *testing.T) {
	t.Setenv("PEERBLADE_API_URL", "")
	t.Setenv("PEERBLADE_SERVER_ID", "")
	t.Setenv("PEERBLADE_AGENT_TOKEN", "")

	if _, err := loadAgentConfig(); err == nil {
		t.Fatal("loadAgentConfig() accepted empty environment")
	}
}

func TestLoadAgentConfigRejectsFastHeartbeat(t *testing.T) {
	t.Setenv("PEERBLADE_API_URL", "http://localhost:4000")
	t.Setenv("PEERBLADE_SERVER_ID", "server-id")
	t.Setenv("PEERBLADE_AGENT_TOKEN", "pbl_token")
	t.Setenv("PEERBLADE_HEARTBEAT_INTERVAL", "1s")

	if _, err := loadAgentConfig(); err == nil {
		t.Fatal("loadAgentConfig() accepted heartbeat below 5s")
	}
}

func TestLoadAgentConfigRejectsFastSnapshot(t *testing.T) {
	t.Setenv("PEERBLADE_API_URL", "http://localhost:4000")
	t.Setenv("PEERBLADE_SERVER_ID", "server-id")
	t.Setenv("PEERBLADE_AGENT_TOKEN", "pbl_token")
	t.Setenv("PEERBLADE_SNAPSHOT_INTERVAL", "1s")

	if _, err := loadAgentConfig(); err == nil {
		t.Fatal("loadAgentConfig() accepted snapshot interval below 10s")
	}
}

func TestLoadAgentConfigRejectsFastCommandPolling(t *testing.T) {
	t.Setenv("PEERBLADE_API_URL", "http://localhost:4000")
	t.Setenv("PEERBLADE_SERVER_ID", "server-id")
	t.Setenv("PEERBLADE_AGENT_TOKEN", "pbl_token")
	t.Setenv("PEERBLADE_COMMAND_INTERVAL", "500ms")

	if _, err := loadAgentConfig(); err == nil {
		t.Fatal("loadAgentConfig() accepted command interval below 1s")
	}
}

func TestLoadAgentConfigLoadsNativeManagement(t *testing.T) {
	t.Setenv("PEERBLADE_API_URL", "http://localhost:4000")
	t.Setenv("PEERBLADE_SERVER_ID", "server-id")
	t.Setenv("PEERBLADE_AGENT_TOKEN", "pbl_token")
	t.Setenv("PEERBLADE_MANAGED_INTERFACE", " peerblade0 ")
	t.Setenv("PEERBLADE_MANAGED_ENDPOINT", " node.example.com:51820 ")
	t.Setenv("PEERBLADE_MANAGED_ADDRESS_CIDR", "10.44.0.1/24")
	t.Setenv("PEERBLADE_MANAGED_DNS", "1.1.1.1, 8.8.8.8")
	t.Setenv("PEERBLADE_STATE_DIRECTORY", "/var/lib/peerblade-agent")

	config, err := loadAgentConfig()
	if err != nil {
		t.Fatalf("loadAgentConfig() error = %v", err)
	}
	if config.managedInterface != "peerblade0" || config.managedEndpoint != "node.example.com:51820" {
		t.Fatalf("unexpected native config: %+v", config)
	}
	if got := strings.Join(config.managedDNS, ","); got != "1.1.1.1,8.8.8.8" {
		t.Fatalf("managed DNS = %q", got)
	}
	if got := strings.Join(config.managedAllowedIPs, ","); got != "0.0.0.0/0" {
		t.Fatalf("default managed AllowedIPs = %q", got)
	}
}

func TestLoadAgentConfigRequiresCompleteNativeManagement(t *testing.T) {
	t.Setenv("PEERBLADE_API_URL", "http://localhost:4000")
	t.Setenv("PEERBLADE_SERVER_ID", "server-id")
	t.Setenv("PEERBLADE_AGENT_TOKEN", "pbl_token")
	t.Setenv("PEERBLADE_MANAGED_INTERFACE", "peerblade0")

	if _, err := loadAgentConfig(); err == nil {
		t.Fatal("loadAgentConfig() accepted incomplete native management config")
	}
}
