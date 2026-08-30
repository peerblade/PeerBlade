package amneziawg

import (
	"strings"
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type fakeRunner struct {
	output []byte
	calls  [][]string
}

func TestClientReadsDisabledAmneziaWGKeepalive(t *testing.T) {
	serverKey, _ := wgtypes.GeneratePrivateKey()
	peerKey, _ := wgtypes.GeneratePrivateKey()
	runner := &fakeRunner{output: []byte(strings.Join([]string{
		serverKey.String() + "\t" + serverKey.PublicKey().String() + "\t51821\t0",
		peerKey.PublicKey().String() + "\t(none)\t(none)\t10.45.0.2/32\t0\t0\t0\toff",
	}, "\n"))}
	client := &Client{command: "awg", runner: runner}

	device, err := client.Device("peerblade-awg0")
	if err != nil {
		t.Fatal(err)
	}
	if len(device.Peers) != 1 {
		t.Fatalf("expected one peer, got %d", len(device.Peers))
	}
	if device.Peers[0].PersistentKeepaliveInterval != 0 {
		t.Fatalf("unexpected keepalive: %s", device.Peers[0].PersistentKeepaliveInterval)
	}
}

func (f *fakeRunner) Run(name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.output, nil
}

func TestClientReadsAmneziaWGDump(t *testing.T) {
	serverKey, _ := wgtypes.GeneratePrivateKey()
	peerKey, _ := wgtypes.GeneratePrivateKey()
	runner := &fakeRunner{output: []byte(strings.Join([]string{
		serverKey.String() + "\t" + serverKey.PublicKey().String() + "\t51821\t0",
		peerKey.PublicKey().String() + "\t(none)\t198.51.100.2:443\t10.45.0.2/32\t1700000000\t1024\t2048\t25",
	}, "\n"))}
	client := &Client{command: "awg", runner: runner}

	device, err := client.Device("peerblade-awg0")
	if err != nil {
		t.Fatal(err)
	}
	if device.ListenPort != 51821 || device.PublicKey != serverKey.PublicKey() || len(device.Peers) != 1 {
		t.Fatalf("unexpected device: %+v", device)
	}
	peer := device.Peers[0]
	if peer.ReceiveBytes != 1024 || peer.TransmitBytes != 2048 || peer.AllowedIPs[0].String() != "10.45.0.2/32" {
		t.Fatalf("unexpected peer: %+v", peer)
	}
}

func TestClientRemovesPeerThroughAwg(t *testing.T) {
	peerKey, _ := wgtypes.GeneratePrivateKey()
	runner := &fakeRunner{}
	client := &Client{command: "awg", runner: runner}
	if err := client.ConfigureDevice("peerblade-awg0", wgtypes.Config{Peers: []wgtypes.PeerConfig{{
		PublicKey: peerKey.PublicKey(), Remove: true,
	}}}); err != nil {
		t.Fatal(err)
	}
	actual := strings.Join(runner.calls[0], " ")
	if !strings.Contains(actual, "awg set peerblade-awg0 peer "+peerKey.PublicKey().String()+" remove") {
		t.Fatalf("unexpected command: %s", actual)
	}
}
