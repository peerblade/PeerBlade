package nativewg

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type deviceClient interface {
	Device(name string) (*wgtypes.Device, error)
	ConfigureDevice(name string, config wgtypes.Config) error
}

type Config struct {
	InterfaceName    string
	Transport        string
	Endpoint         string
	AddressCIDR      string
	DNS              []string
	ClientAllowedIPs []string
	KeepaliveSeconds int
	Amnezia          AmneziaParameters
}

type AmneziaParameters struct {
	Jc   int
	Jmin int
	Jmax int
	S1   int
	S2   int
	H1   int64
	H2   int64
	H3   int64
	H4   int64
}

type CreatedPeer struct {
	Name          string `json:"name"`
	PublicKey     string `json:"publicKey"`
	Configuration string `json:"configuration"`
}

type Manager struct {
	client  deviceClient
	store   *Store
	config  Config
	network *net.IPNet
	mu      sync.Mutex
}

var interfaceNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,15}$`)

func NewManager(client deviceClient, store *Store, config Config) (*Manager, error) {
	if client == nil || store == nil {
		return nil, errors.New("WireGuard client and state store are required")
	}
	config.InterfaceName = strings.TrimSpace(config.InterfaceName)
	config.Transport = strings.TrimSpace(config.Transport)
	if config.Transport == "" {
		config.Transport = "wireguard"
	}
	if config.Transport != "wireguard" && config.Transport != "amneziawg" {
		return nil, errors.New("managed transport must be wireguard or amneziawg")
	}
	if config.Transport == "amneziawg" {
		if err := validateAmneziaParameters(config.Amnezia); err != nil {
			return nil, err
		}
	}
	config.Endpoint = strings.TrimSpace(config.Endpoint)
	if config.InterfaceName == "" || config.Endpoint == "" {
		return nil, errors.New("managed interface and endpoint are required")
	}
	if !interfaceNamePattern.MatchString(config.InterfaceName) {
		return nil, errors.New("managed interface has an invalid Linux interface name")
	}
	hostValue, portValue, err := net.SplitHostPort(config.Endpoint)
	if err != nil || strings.TrimSpace(hostValue) == "" {
		return nil, errors.New("managed endpoint must use host:port format")
	}
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return nil, errors.New("managed endpoint port must be between 1 and 65535")
	}
	ip, network, err := net.ParseCIDR(config.AddressCIDR)
	if err != nil || ip.To4() == nil {
		return nil, errors.New("managed address CIDR must be a valid IPv4 network")
	}
	ones, bits := network.Mask.Size()
	if bits != 32 || ones != 24 {
		return nil, errors.New("managed address CIDR must use a /24 IPv4 network")
	}
	if ip.To4()[3] != 1 {
		return nil, errors.New("managed address CIDR must use the first host address (.1/24)")
	}
	if len(config.ClientAllowedIPs) == 0 {
		return nil, errors.New("at least one client AllowedIPs network is required")
	}
	if _, err := parseNetworks(config.ClientAllowedIPs); err != nil {
		return nil, fmt.Errorf("validate client AllowedIPs: %w", err)
	}
	network.IP = ip.Mask(network.Mask)
	if config.KeepaliveSeconds <= 0 {
		config.KeepaliveSeconds = 25
	}
	device, err := client.Device(config.InterfaceName)
	if err != nil {
		return nil, fmt.Errorf("read managed interface: %w", err)
	}
	if device.PublicKey == (wgtypes.Key{}) {
		return nil, errors.New("managed interface does not have a public key")
	}

	return &Manager{client: client, store: store, config: config, network: network}, nil
}

func (m *Manager) CreatePeer(name string) (CreatedPeer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return CreatedPeer{}, errors.New("peer name must contain 1 to 100 characters")
	}
	device, err := m.client.Device(m.config.InterfaceName)
	if err != nil {
		return CreatedPeer{}, fmt.Errorf("read managed interface: %w", err)
	}
	state, err := m.loadState()
	if err != nil {
		return CreatedPeer{}, err
	}
	address, err := m.nextAddress(state)
	if err != nil {
		return CreatedPeer{}, err
	}
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return CreatedPeer{}, fmt.Errorf("generate peer private key: %w", err)
	}
	presharedKey, err := wgtypes.GenerateKey()
	if err != nil {
		return CreatedPeer{}, fmt.Errorf("generate peer preshared key: %w", err)
	}
	peer := Peer{
		Name:         name,
		Address:      address,
		PrivateKey:   privateKey.String(),
		PublicKey:    privateKey.PublicKey().String(),
		PresharedKey: presharedKey.String(),
		AllowedIPs:   []string{address + "/32"},
		Enabled:      true,
	}
	if err := m.applyPeer(peer); err != nil {
		return CreatedPeer{}, err
	}
	state.Peers = append(state.Peers, peer)
	if err := m.store.Save(state); err != nil {
		_ = m.removePeer(peer.PublicKey)
		return CreatedPeer{}, err
	}
	configuration := m.renderConfiguration(peer, device.PublicKey.String())

	return CreatedPeer{Name: name, PublicKey: peer.PublicKey, Configuration: configuration}, nil
}

func (m *Manager) SetEnabled(publicKey string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, err := m.loadState()
	if err != nil {
		return err
	}
	index := findPeer(state.Peers, publicKey)
	if index < 0 {
		return errors.New("peer is not managed by PeerBlade")
	}
	previousPeer := state.Peers[index]
	state.Peers[index].Enabled = enabled
	if err := m.applyPeer(state.Peers[index]); err != nil {
		return err
	}
	if err := m.store.Save(state); err != nil {
		_ = m.applyPeer(previousPeer)
		return err
	}
	return nil
}

func (m *Manager) DeletePeer(publicKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, err := m.loadState()
	if err != nil {
		return err
	}
	index := findPeer(state.Peers, publicKey)
	if index < 0 {
		return errors.New("peer is not managed by PeerBlade")
	}
	deletedPeer := state.Peers[index]
	if err := m.removePeer(publicKey); err != nil {
		return err
	}
	state.Peers = append(state.Peers[:index], state.Peers[index+1:]...)
	if err := m.store.Save(state); err != nil {
		_ = m.applyPeer(deletedPeer)
		return err
	}
	return nil
}

func (m *Manager) Configuration(publicKey string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, err := m.loadState()
	if err != nil {
		return "", err
	}
	index := findPeer(state.Peers, publicKey)
	if index < 0 {
		return "", errors.New("peer is not managed by PeerBlade")
	}
	return m.configuration(state.Peers[index])
}

func (m *Manager) Reconcile() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, err := m.loadState()
	if err != nil {
		return err
	}
	for _, peer := range state.Peers {
		if err := m.applyPeer(peer); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) Peers() ([]Peer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, err := m.loadState()
	if err != nil {
		return nil, err
	}
	return state.Peers, nil
}

func (m *Manager) applyPeer(peer Peer) error {
	publicKey, err := wgtypes.ParseKey(peer.PublicKey)
	if err != nil {
		return fmt.Errorf("parse peer public key: %w", err)
	}
	presharedKey, err := wgtypes.ParseKey(peer.PresharedKey)
	if err != nil {
		return fmt.Errorf("parse peer preshared key: %w", err)
	}
	allowedIPs := []net.IPNet{}
	if peer.Enabled {
		allowedIPs, err = parseNetworks(peer.AllowedIPs)
		if err != nil {
			return err
		}
	}
	if err := m.client.ConfigureDevice(m.config.InterfaceName, wgtypes.Config{Peers: []wgtypes.PeerConfig{{
		PublicKey: publicKey, PresharedKey: &presharedKey, ReplaceAllowedIPs: true, AllowedIPs: allowedIPs,
	}}}); err != nil {
		return fmt.Errorf("configure managed peer: %w", err)
	}
	return nil
}

func (m *Manager) loadState() (State, error) {
	state, err := m.store.Load()
	if err != nil {
		return State{}, err
	}
	if err := validateState(state, m.network); err != nil {
		return State{}, err
	}
	return state, nil
}

func validateState(state State, network *net.IPNet) error {
	addresses := make(map[string]struct{}, len(state.Peers))
	publicKeys := make(map[string]struct{}, len(state.Peers))
	for index, peer := range state.Peers {
		if strings.TrimSpace(peer.Name) == "" || utf8.RuneCountInString(peer.Name) > 100 {
			return fmt.Errorf("peer state item %d has an invalid name", index)
		}
		privateKey, err := wgtypes.ParseKey(peer.PrivateKey)
		if err != nil {
			return fmt.Errorf("peer state item %d has an invalid private key: %w", index, err)
		}
		publicKey, err := wgtypes.ParseKey(peer.PublicKey)
		if err != nil || privateKey.PublicKey() != publicKey {
			return fmt.Errorf("peer state item %d has an invalid public key", index)
		}
		if _, err := wgtypes.ParseKey(peer.PresharedKey); err != nil {
			return fmt.Errorf("peer state item %d has an invalid preshared key: %w", index, err)
		}
		address := net.ParseIP(peer.Address).To4()
		if address == nil || !network.Contains(address) || address[3] < 2 || address[3] > 254 {
			return fmt.Errorf("peer state item %d has an invalid managed address", index)
		}
		if len(peer.AllowedIPs) != 1 || peer.AllowedIPs[0] != peer.Address+"/32" {
			return fmt.Errorf("peer state item %d has invalid AllowedIPs", index)
		}
		if _, exists := addresses[peer.Address]; exists {
			return fmt.Errorf("peer state contains duplicate address %s", peer.Address)
		}
		if _, exists := publicKeys[peer.PublicKey]; exists {
			return errors.New("peer state contains a duplicate public key")
		}
		addresses[peer.Address] = struct{}{}
		publicKeys[peer.PublicKey] = struct{}{}
	}
	return nil
}

func (m *Manager) removePeer(publicKeyValue string) error {
	publicKey, err := wgtypes.ParseKey(publicKeyValue)
	if err != nil {
		return fmt.Errorf("parse peer public key: %w", err)
	}
	if err := m.client.ConfigureDevice(m.config.InterfaceName, wgtypes.Config{Peers: []wgtypes.PeerConfig{{
		PublicKey: publicKey, Remove: true,
	}}}); err != nil {
		return fmt.Errorf("remove managed peer: %w", err)
	}
	return nil
}

func (m *Manager) configuration(peer Peer) (string, error) {
	device, err := m.client.Device(m.config.InterfaceName)
	if err != nil {
		return "", fmt.Errorf("read managed interface: %w", err)
	}
	return m.renderConfiguration(peer, device.PublicKey.String()), nil
}

func (m *Manager) renderConfiguration(peer Peer, serverPublicKey string) string {
	interfaceLines := []string{
		"[Interface]",
		"PrivateKey = " + peer.PrivateKey,
		"Address = " + peer.Address + "/32",
	}
	if m.config.Transport == "amneziawg" {
		interfaceLines = append(interfaceLines,
			fmt.Sprintf("Jc = %d", m.config.Amnezia.Jc),
			fmt.Sprintf("Jmin = %d", m.config.Amnezia.Jmin),
			fmt.Sprintf("Jmax = %d", m.config.Amnezia.Jmax),
			fmt.Sprintf("S1 = %d", m.config.Amnezia.S1),
			fmt.Sprintf("S2 = %d", m.config.Amnezia.S2),
			fmt.Sprintf("H1 = %d", m.config.Amnezia.H1),
			fmt.Sprintf("H2 = %d", m.config.Amnezia.H2),
			fmt.Sprintf("H3 = %d", m.config.Amnezia.H3),
			fmt.Sprintf("H4 = %d", m.config.Amnezia.H4),
		)
	}
	if len(m.config.DNS) > 0 {
		interfaceLines = append(interfaceLines, "DNS = "+strings.Join(m.config.DNS, ", "))
	}
	peerLines := []string{
		"[Peer]",
		"PublicKey = " + serverPublicKey,
		"PresharedKey = " + peer.PresharedKey,
		"AllowedIPs = " + strings.Join(m.config.ClientAllowedIPs, ", "),
		"Endpoint = " + m.config.Endpoint,
		fmt.Sprintf("PersistentKeepalive = %d", m.config.KeepaliveSeconds),
	}

	return strings.Join(append(interfaceLines, append([]string{""}, peerLines...)...), "\n") + "\n"
}

func validateAmneziaParameters(parameters AmneziaParameters) error {
	if parameters.Jc < 1 || parameters.Jc > 128 {
		return errors.New("AmneziaWG Jc must be between 1 and 128")
	}
	if parameters.Jmin < 1 || parameters.Jmax > 1280 || parameters.Jmin > parameters.Jmax {
		return errors.New("AmneziaWG junk packet range is invalid")
	}
	if parameters.S1 < 0 || parameters.S1 > 1132 || parameters.S2 < 0 || parameters.S2 > 1188 {
		return errors.New("AmneziaWG S1/S2 values are invalid")
	}
	if parameters.S1+56 == parameters.S2 {
		return errors.New("AmneziaWG S1 + 56 must not equal S2")
	}
	headers := []int64{parameters.H1, parameters.H2, parameters.H3, parameters.H4}
	seen := map[int64]bool{}
	for _, header := range headers {
		if header < 5 || header > 2147483647 || seen[header] {
			return errors.New("AmneziaWG H1-H4 must be unique values between 5 and 2147483647")
		}
		seen[header] = true
	}
	return nil
}

func (m *Manager) nextAddress(state State) (string, error) {
	used := map[string]bool{}
	for _, peer := range state.Peers {
		used[peer.Address] = true
	}
	base := m.network.IP.To4()
	for host := 2; host < 255; host++ {
		candidate := net.IPv4(base[0], base[1], base[2], byte(host)).String()
		if m.network.Contains(net.ParseIP(candidate)) && !used[candidate] {
			return candidate, nil
		}
	}
	return "", errors.New("managed address pool is exhausted")
}

func findPeer(peers []Peer, publicKey string) int {
	for index, peer := range peers {
		if peer.PublicKey == publicKey {
			return index
		}
	}
	return -1
}

func parseNetworks(values []string) ([]net.IPNet, error) {
	result := make([]net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("parse allowed IP %q: %w", value, err)
		}
		result = append(result, *network)
	}
	return result, nil
}
