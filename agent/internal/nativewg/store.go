package nativewg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const stateVersion = 1

type Peer struct {
	Name         string   `json:"name"`
	Address      string   `json:"address"`
	PrivateKey   string   `json:"privateKey"`
	PublicKey    string   `json:"publicKey"`
	PresharedKey string   `json:"presharedKey"`
	AllowedIPs   []string `json:"allowedIps"`
	Enabled      bool     `json:"enabled"`
}

type State struct {
	Version int    `json:"version"`
	Peers   []Peer `json:"peers"`
}

type Store struct {
	directory string
	path      string
}

func NewStore(directory string) (*Store, error) {
	if directory == "" {
		return nil, errors.New("state directory is required")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, fmt.Errorf("inspect state directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("state directory must be a real directory")
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, fmt.Errorf("secure state directory: %w", err)
	}

	return &Store{
		directory: directory,
		path:      filepath.Join(directory, "peers.json"),
	}, nil
}

func (s *Store) Load() (State, error) {
	info, err := os.Lstat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return State{Version: stateVersion, Peers: []Peer{}}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("inspect peer state: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return State{}, errors.New("peer state must be a regular file")
	}
	if info.Mode().Perm() != 0o600 {
		if err := os.Chmod(s.path, 0o600); err != nil {
			return State{}, fmt.Errorf("secure peer state: %w", err)
		}
	}

	file, err := os.Open(s.path)
	if err != nil {
		return State{}, fmt.Errorf("open peer state: %w", err)
	}
	defer file.Close()

	var state State
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return State{}, fmt.Errorf("decode peer state: %w", err)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return State{}, errors.New("peer state contains trailing data")
	}
	if state.Version != stateVersion {
		return State{}, fmt.Errorf("unsupported peer state version %d", state.Version)
	}
	if state.Peers == nil {
		state.Peers = []Peer{}
	}

	return state, nil
}

func (s *Store) Save(state State) error {
	state.Version = stateVersion
	temporary, err := os.CreateTemp(s.directory, ".peers-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary peer state: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("secure temporary peer state: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(state); err != nil {
		temporary.Close()
		return fmt.Errorf("encode peer state: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync peer state: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close peer state: %w", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("replace peer state: %w", err)
	}
	directory, err := os.Open(s.directory)
	if err != nil {
		return fmt.Errorf("open state directory for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync state directory: %w", err)
	}

	return nil
}
