package nativewg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type fakeDeviceClient struct {
	device  *wgtypes.Device
	configs []wgtypes.Config
}

func (f *fakeDeviceClient) Device(string) (*wgtypes.Device, error) {
	return f.device, nil
}

func (f *fakeDeviceClient) ConfigureDevice(_ string, config wgtypes.Config) error {
	f.configs = append(f.configs, config)
	return nil
}

func newTestManager(t *testing.T) (*Manager, *fakeDeviceClient, *Store) {
	t.Helper()
	serverPrivateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeDeviceClient{device: &wgtypes.Device{
		Name:      "peerblade0",
		PublicKey: serverPrivateKey.PublicKey(),
	}}
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(client, store, Config{
		InterfaceName:    "peerblade0",
		Endpoint:         "node.example.com:51820",
		AddressCIDR:      "10.44.0.1/24",
		DNS:              []string{"1.1.1.1"},
		ClientAllowedIPs: []string{"0.0.0.0/0", "::/0"},
		KeepaliveSeconds: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	return manager, client, store
}

func TestManagerCreateDisableEnableAndDeletePeer(t *testing.T) {
	manager, client, store := newTestManager(t)

	created, err := manager.CreatePeer("Redmi 15 Pro")
	if err != nil {
		t.Fatalf("CreatePeer() error = %v", err)
	}
	if created.Name != "Redmi 15 Pro" || created.PublicKey == "" {
		t.Fatalf("unexpected created peer: %+v", created)
	}
	for _, expected := range []string{
		"Address = 10.44.0.2/32",
		"DNS = 1.1.1.1",
		"Endpoint = node.example.com:51820",
		"AllowedIPs = 0.0.0.0/0, ::/0",
	} {
		if !strings.Contains(created.Configuration, expected) {
			t.Fatalf("configuration does not contain %q:\n%s", expected, created.Configuration)
		}
	}

	state, err := store.Load()
	if err != nil || len(state.Peers) != 1 || !state.Peers[0].Enabled {
		t.Fatalf("unexpected stored state: %+v, error = %v", state, err)
	}
	if state.Peers[0].PrivateKey == "" || state.Peers[0].PresharedKey == "" {
		t.Fatal("local peer secrets were not stored")
	}
	assertAllowedIPs(t, client.configs[len(client.configs)-1], "10.44.0.2/32")

	if err := manager.SetEnabled(created.PublicKey, false); err != nil {
		t.Fatalf("SetEnabled(false) error = %v", err)
	}
	assertAllowedIPs(t, client.configs[len(client.configs)-1])

	if err := manager.SetEnabled(created.PublicKey, true); err != nil {
		t.Fatalf("SetEnabled(true) error = %v", err)
	}
	assertAllowedIPs(t, client.configs[len(client.configs)-1], "10.44.0.2/32")

	if err := manager.DeletePeer(created.PublicKey); err != nil {
		t.Fatalf("DeletePeer() error = %v", err)
	}
	lastPeerConfig := client.configs[len(client.configs)-1].Peers[0]
	if !lastPeerConfig.Remove {
		t.Fatal("delete did not remove the WireGuard peer")
	}
	state, err = store.Load()
	if err != nil || len(state.Peers) != 0 {
		t.Fatalf("peer remains in local state: %+v, error = %v", state, err)
	}
}

func TestManagerAllocatesSequentialAddresses(t *testing.T) {
	manager, _, _ := newTestManager(t)
	first, err := manager.CreatePeer("first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.CreatePeer("second")
	if err != nil {
		t.Fatal(err)
	}
	firstConfiguration, _ := manager.Configuration(first.PublicKey)
	secondConfiguration, _ := manager.Configuration(second.PublicKey)
	if !strings.Contains(firstConfiguration, "10.44.0.2/32") || !strings.Contains(secondConfiguration, "10.44.0.3/32") {
		t.Fatalf("unexpected allocated addresses:\n%s\n%s", firstConfiguration, secondConfiguration)
	}
}

func TestNewManagerRejectsNon24Network(t *testing.T) {
	_, client, store := newTestManager(t)
	_, err := NewManager(client, store, Config{
		InterfaceName:    "peerblade0",
		Endpoint:         "node.example.com:51820",
		AddressCIDR:      "10.44.0.1/16",
		ClientAllowedIPs: []string{"0.0.0.0/0"},
	})
	if err == nil || !strings.Contains(err.Error(), "/24") {
		t.Fatalf("NewManager() error = %v", err)
	}
}

func TestStoreUsesPrivatePermissions(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	store, err := NewStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(State{}); err != nil {
		t.Fatal(err)
	}
	assertMode(t, directory, 0o700)
	assertMode(t, filepath.Join(directory, "peers.json"), 0o600)
}

func TestStoreRejectsSymlinkedStateFile(t *testing.T) {
	directory := t.TempDir()
	store, err := NewStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target.json")
	if err := os.WriteFile(target, []byte(`{"version":1,"peers":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(directory, "peers.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("Load() error = %v", err)
	}
}

func assertAllowedIPs(t *testing.T, config wgtypes.Config, expected ...string) {
	t.Helper()
	if len(config.Peers) != 1 {
		t.Fatalf("peer configs = %d", len(config.Peers))
	}
	actual := make([]string, 0, len(config.Peers[0].AllowedIPs))
	for _, network := range config.Peers[0].AllowedIPs {
		actual = append(actual, network.String())
	}
	if strings.Join(actual, ",") != strings.Join(expected, ",") {
		t.Fatalf("AllowedIPs = %v, want %v", actual, expected)
	}
}

func assertMode(t *testing.T, path string, expected os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if actual := info.Mode().Perm(); actual != expected {
		t.Fatalf("%s mode = %o, want %o", path, actual, expected)
	}
}
