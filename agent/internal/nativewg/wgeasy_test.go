package nativewg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestLoadWGEasyStateValidatesAndSortsPeers(t *testing.T) {
	serverPrivateKey := generatePrivateKey(t)
	firstPrivateKey := generatePrivateKey(t)
	secondPrivateKey := generatePrivateKey(t)
	document := map[string]interface{}{
		"server": map[string]interface{}{
			"address":    "10.8.0.1",
			"privateKey": serverPrivateKey.String(),
			"publicKey":  serverPrivateKey.PublicKey().String(),
		},
		"clients": map[string]interface{}{
			"later": map[string]interface{}{
				"name":         "Laptop",
				"address":      "10.8.0.12",
				"privateKey":   secondPrivateKey.String(),
				"publicKey":    secondPrivateKey.PublicKey().String(),
				"preSharedKey": generatePresharedKey(t).String(),
				"enabled":      false,
				"createdAt":    "2026-01-01T00:00:00.000Z",
			},
			"first": map[string]interface{}{
				"name":         "Phone",
				"address":      "10.8.0.2",
				"privateKey":   firstPrivateKey.String(),
				"publicKey":    firstPrivateKey.PublicKey().String(),
				"preSharedKey": generatePresharedKey(t).String(),
				"enabled":      true,
			},
		},
	}
	path := writeWGEasyDocument(t, document)

	state, summary, err := LoadWGEasyState(path)
	if err != nil {
		t.Fatalf("LoadWGEasyState() error = %v", err)
	}
	if summary.ServerAddress != "10.8.0.1/24" || summary.ServerPublicKey != serverPrivateKey.PublicKey().String() {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if summary.PeerCount != 2 || summary.EnabledPeers != 1 || summary.DisabledPeers != 1 {
		t.Fatalf("unexpected peer counts: %+v", summary)
	}
	if len(state.Peers) != 2 || state.Peers[0].Address != "10.8.0.2" || state.Peers[1].Address != "10.8.0.12" {
		t.Fatalf("peers were not sorted by address: %+v", state.Peers)
	}
	if !state.Peers[0].Enabled || state.Peers[1].Enabled {
		t.Fatalf("enabled state was not preserved: %+v", state.Peers)
	}
}

func TestLoadWGEasyStateRejectsMismatchedClientKeys(t *testing.T) {
	serverPrivateKey := generatePrivateKey(t)
	clientPrivateKey := generatePrivateKey(t)
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
				"publicKey":    generatePrivateKey(t).PublicKey().String(),
				"preSharedKey": generatePresharedKey(t).String(),
				"enabled":      true,
			},
		},
	}

	_, _, err := LoadWGEasyState(writeWGEasyDocument(t, document))
	if err == nil || !strings.Contains(err.Error(), "invalid public key") {
		t.Fatalf("LoadWGEasyState() error = %v", err)
	}
}

func TestLoadWGEasyStateRejectsSymlink(t *testing.T) {
	target := filepath.Join(t.TempDir(), "wg0.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "wg0.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	_, _, err := LoadWGEasyState(link)
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("LoadWGEasyState() error = %v", err)
	}
}

func generatePrivateKey(t *testing.T) wgtypes.Key {
	t.Helper()
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func generatePresharedKey(t *testing.T) wgtypes.Key {
	t.Helper()
	key, err := wgtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func writeWGEasyDocument(t *testing.T, document interface{}) string {
	t.Helper()
	contents, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "wg0.json")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
