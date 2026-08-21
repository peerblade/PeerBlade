package nativewg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const maxWGEasyStateSize = 4 << 20

type WGEasyImportSummary struct {
	ServerAddress   string `json:"serverAddress"`
	ServerPublicKey string `json:"serverPublicKey"`
	PeerCount       int    `json:"peerCount"`
	EnabledPeers    int    `json:"enabledPeers"`
	DisabledPeers   int    `json:"disabledPeers"`
}

type wgEasyDocument struct {
	Server struct {
		Address    string `json:"address"`
		PrivateKey string `json:"privateKey"`
		PublicKey  string `json:"publicKey"`
	} `json:"server"`
	Clients map[string]wgEasyClient `json:"clients"`
}

type wgEasyClient struct {
	Name         string `json:"name"`
	Address      string `json:"address"`
	PrivateKey   string `json:"privateKey"`
	PublicKey    string `json:"publicKey"`
	PresharedKey string `json:"preSharedKey"`
	Enabled      bool   `json:"enabled"`
}

func LoadWGEasyState(path string) (State, WGEasyImportSummary, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return State{}, WGEasyImportSummary{}, fmt.Errorf("inspect wg-easy state: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return State{}, WGEasyImportSummary{}, errors.New("wg-easy state must be a regular file")
	}
	if info.Size() > maxWGEasyStateSize {
		return State{}, WGEasyImportSummary{}, errors.New("wg-easy state exceeds the 4 MiB limit")
	}

	file, err := os.Open(path)
	if err != nil {
		return State{}, WGEasyImportSummary{}, fmt.Errorf("open wg-easy state: %w", err)
	}
	defer file.Close()

	var document wgEasyDocument
	decoder := json.NewDecoder(io.LimitReader(file, maxWGEasyStateSize+1))
	if err := decoder.Decode(&document); err != nil {
		return State{}, WGEasyImportSummary{}, fmt.Errorf("decode wg-easy state: %w", err)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return State{}, WGEasyImportSummary{}, errors.New("wg-easy state contains trailing data")
	}

	serverPrivateKey, err := wgtypes.ParseKey(strings.TrimSpace(document.Server.PrivateKey))
	if err != nil {
		return State{}, WGEasyImportSummary{}, fmt.Errorf("validate wg-easy server private key: %w", err)
	}
	serverPublicKey, err := wgtypes.ParseKey(strings.TrimSpace(document.Server.PublicKey))
	if err != nil || serverPrivateKey.PublicKey() != serverPublicKey {
		return State{}, WGEasyImportSummary{}, errors.New("wg-easy server public key does not match its private key")
	}

	serverAddress := net.ParseIP(strings.TrimSpace(document.Server.Address)).To4()
	if serverAddress == nil || serverAddress[3] != 1 {
		return State{}, WGEasyImportSummary{}, errors.New("wg-easy server address must be the first host of an IPv4 /24")
	}
	network := &net.IPNet{IP: serverAddress.Mask(net.CIDRMask(24, 32)), Mask: net.CIDRMask(24, 32)}

	state := State{Version: stateVersion, Peers: make([]Peer, 0, len(document.Clients))}
	summary := WGEasyImportSummary{
		ServerAddress:   serverAddress.String() + "/24",
		ServerPublicKey: serverPublicKey.String(),
		PeerCount:       len(document.Clients),
	}
	for _, client := range document.Clients {
		address := net.ParseIP(strings.TrimSpace(client.Address)).To4()
		if address == nil {
			return State{}, WGEasyImportSummary{}, errors.New("wg-easy client contains an invalid IPv4 address")
		}
		peer := Peer{
			Name:         strings.TrimSpace(client.Name),
			Address:      address.String(),
			PrivateKey:   strings.TrimSpace(client.PrivateKey),
			PublicKey:    strings.TrimSpace(client.PublicKey),
			PresharedKey: strings.TrimSpace(client.PresharedKey),
			AllowedIPs:   []string{address.String() + "/32"},
			Enabled:      client.Enabled,
		}
		state.Peers = append(state.Peers, peer)
		if peer.Enabled {
			summary.EnabledPeers++
		} else {
			summary.DisabledPeers++
		}
	}
	sort.Slice(state.Peers, func(left, right int) bool {
		return bytes.Compare(net.ParseIP(state.Peers[left].Address).To4(), net.ParseIP(state.Peers[right].Address).To4()) < 0
	})
	if err := validateState(state, network); err != nil {
		return State{}, WGEasyImportSummary{}, fmt.Errorf("validate imported peers: %w", err)
	}

	return state, summary, nil
}
