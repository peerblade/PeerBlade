package wireguard

import (
	"errors"
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type fakeConfigurator struct {
	name   string
	config wgtypes.Config
	err    error
}

func (f *fakeConfigurator) ConfigureDevice(name string, config wgtypes.Config) error {
	f.name = name
	f.config = config
	return f.err
}

func TestControllerReplacesOnlyPeerAllowedIPs(t *testing.T) {
	client := &fakeConfigurator{}
	controller := NewController(client)
	publicKey := testKey(7).String()

	if err := controller.SetPeerAllowedIPs(
		"wg0",
		publicKey,
		[]string{"10.8.0.2/32"},
	); err != nil {
		t.Fatalf("SetPeerAllowedIPs() error = %v", err)
	}

	if client.name != "wg0" || len(client.config.Peers) != 1 {
		t.Fatalf("unexpected config: name=%q config=%+v", client.name, client.config)
	}
	peer := client.config.Peers[0]
	if peer.PublicKey.String() != publicKey || !peer.ReplaceAllowedIPs || len(peer.AllowedIPs) != 1 {
		t.Fatalf("unexpected peer config: %+v", peer)
	}
	if peer.Remove || peer.PresharedKey != nil || peer.Endpoint != nil {
		t.Fatalf("controller changed unsafe peer fields: %+v", peer)
	}
}

func TestControllerDisablesPeerWithEmptyAllowedIPs(t *testing.T) {
	client := &fakeConfigurator{}
	controller := NewController(client)

	if err := controller.SetPeerAllowedIPs("wg0", testKey(8).String(), nil); err != nil {
		t.Fatalf("SetPeerAllowedIPs() error = %v", err)
	}
	if got := client.config.Peers[0].AllowedIPs; len(got) != 0 {
		t.Fatalf("AllowedIPs = %v, want empty", got)
	}
}

func TestControllerRejectsInvalidAllowedIP(t *testing.T) {
	controller := NewController(&fakeConfigurator{})

	if err := controller.SetPeerAllowedIPs(
		"wg0",
		testKey(9).String(),
		[]string{"not-a-cidr"},
	); err == nil {
		t.Fatal("SetPeerAllowedIPs() accepted invalid CIDR")
	}
}

func TestControllerWrapsConfigureError(t *testing.T) {
	controller := NewController(&fakeConfigurator{err: errors.New("permission denied")})

	if err := controller.SetPeerAllowedIPs("wg0", testKey(10).String(), nil); err == nil {
		t.Fatal("SetPeerAllowedIPs() returned nil")
	}
}
