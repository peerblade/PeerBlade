package controlplane

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/peerblade/PeerBlade/agent/internal/wireguard"
)

func TestClientRegistersAgent(t *testing.T) {
	httpClient := newTestHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/agent/v1/register" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if got, want := request.Header.Get("Authorization"), "Bearer pbl_secret"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}

		var body struct {
			ServerID     string   `json:"serverId"`
			Version      string   `json:"version"`
			Capabilities []string `json:"capabilities"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.ServerID != "server-id" || body.Version != "0.1.0" || len(body.Capabilities) != 1 {
			t.Fatalf("unexpected request body: %#v", body)
		}

		return jsonResponse(http.StatusOK, `{
			"id":"agent-id",
			"serverId":"server-id",
			"status":"online",
			"version":"0.1.0",
			"registeredAt":"2026-08-01T00:00:00.000Z",
			"lastSeenAt":"2026-08-01T00:00:00.000Z"
		}`), nil
	})

	client, err := NewClient("http://control-plane.test/", "pbl_secret", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	agent, err := client.Register(
		context.Background(),
		"server-id",
		"0.1.0",
		[]string{"wireguard_snapshot"},
	)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if agent.ID != "agent-id" || agent.Status != "online" {
		t.Fatalf("unexpected agent response: %+v", agent)
	}
}

func TestClientSendsHeartbeat(t *testing.T) {
	httpClient := newTestHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/agent/v1/heartbeat" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		return jsonResponse(http.StatusOK, `{
			"id":"agent-id",
			"serverId":"server-id",
			"status":"online",
			"version":"0.1.0",
			"registeredAt":"2026-08-01T00:00:00.000Z",
			"lastSeenAt":"2026-08-01T00:00:30.000Z"
		}`), nil
	})

	client, err := NewClient("http://control-plane.test", "pbl_secret", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	agent, err := client.Heartbeat(context.Background(), "0.1.0")
	if err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}
	if agent.LastSeenAt != "2026-08-01T00:00:30.000Z" {
		t.Fatalf("LastSeenAt = %q", agent.LastSeenAt)
	}
}

func TestClientUploadsSnapshot(t *testing.T) {
	collectedAt := time.Date(2026, time.August, 1, 0, 1, 0, 0, time.UTC)
	httpClient := newTestHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/agent/v1/snapshot" {
			t.Fatalf("path = %q", request.URL.Path)
		}

		var body wireguard.Snapshot
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.SchemaVersion != 1 || !body.CollectedAt.Equal(collectedAt) {
			t.Fatalf("unexpected snapshot: %+v", body)
		}

		return jsonResponse(http.StatusOK, `{
			"id":"agent-id",
			"serverId":"server-id",
			"status":"online",
			"version":"0.1.0",
			"registeredAt":"2026-08-01T00:00:00.000Z",
			"lastSeenAt":"2026-08-01T00:01:00.000Z",
			"lastSnapshotAt":"2026-08-01T00:01:00.000Z"
		}`), nil
	})

	client, err := NewClient("http://control-plane.test", "pbl_secret", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	agent, err := client.Snapshot(context.Background(), wireguard.Snapshot{
		SchemaVersion: 1,
		CollectedAt:   collectedAt,
		Devices:       []wireguard.DeviceSnapshot{},
	})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if agent.LastSnapshotAt == nil || *agent.LastSnapshotAt != "2026-08-01T00:01:00.000Z" {
		t.Fatalf("LastSnapshotAt = %v", agent.LastSnapshotAt)
	}
}

func TestClientClaimsAndCompletesCommand(t *testing.T) {
	requests := 0
	httpClient := newTestHTTPClient(func(request *http.Request) (*http.Response, error) {
		requests++
		switch request.URL.Path {
		case "/api/agent/v1/commands/next":
			if request.Method != http.MethodGet {
				t.Fatalf("next method = %q", request.Method)
			}
			return jsonResponse(http.StatusOK, `{"command":{
				"id":"command-id",
				"type":"disable_peer",
				"interfaceName":"wg0",
				"publicKey":"public-key",
				"allowedIps":[]
			}
			}`), nil
		case "/api/agent/v1/commands/command-id/result":
			if request.Method != http.MethodPost {
				t.Fatalf("result method = %q", request.Method)
			}
			var result AgentCommandResult
			if err := json.NewDecoder(request.Body).Decode(&result); err != nil {
				t.Fatalf("decode result: %v", err)
			}
			if result.Status != "succeeded" {
				t.Fatalf("unexpected result: %+v", result)
			}
			return jsonResponse(http.StatusNoContent, ""), nil
		default:
			t.Fatalf("unexpected path = %q", request.URL.Path)
			return nil, nil
		}
	})
	client, err := NewClient("http://control-plane.test", "pbl_secret", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	command, err := client.NextCommand(context.Background())
	if err != nil || command == nil || command.Type != "disable_peer" {
		t.Fatalf("NextCommand() = %+v, %v", command, err)
	}
	if err := client.CompleteCommand(
		context.Background(),
		command.ID,
		AgentCommandResult{Status: "succeeded"},
	); err != nil {
		t.Fatalf("CompleteCommand() error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestClientReturnsNilWhenNoCommandIsAvailable(t *testing.T) {
	httpClient := newTestHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Path != "/api/agent/v1/commands/next" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}

		return jsonResponse(http.StatusOK, `{"command":null}`), nil
	})
	client, err := NewClient("http://control-plane.test", "pbl_secret", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	command, err := client.NextCommand(context.Background())
	if err != nil {
		t.Fatalf("NextCommand() error = %v", err)
	}
	if command != nil {
		t.Fatalf("NextCommand() = %+v, want nil", command)
	}
}

func TestClientReturnsSafeAPIError(t *testing.T) {
	httpClient := newTestHTTPClient(func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(
			http.StatusUnauthorized,
			`{"message":"Invalid agent credentials"}`,
		), nil
	})

	client, err := NewClient("http://control-plane.test", "pbl_secret", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Heartbeat(context.Background(), "0.1.0")
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("Heartbeat() error = %T %v, want *APIError", err, err)
	}
	if apiError.StatusCode != http.StatusUnauthorized || strings.Contains(err.Error(), "pbl_secret") {
		t.Fatalf("unexpected API error: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func newTestHTTPClient(roundTrip roundTripFunc) *http.Client {
	return &http.Client{Transport: roundTrip}
}

func jsonResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestNewClientValidatesConfiguration(t *testing.T) {
	client := &http.Client{}

	if _, err := NewClient("localhost:4000", "token", client); err == nil {
		t.Fatal("NewClient() accepted relative URL")
	}
	if _, err := NewClient("http://localhost:4000", "", client); err == nil {
		t.Fatal("NewClient() accepted empty token")
	}
	if _, err := NewClient("http://localhost:4000", "token", nil); err == nil {
		t.Fatal("NewClient() accepted nil HTTP client")
	}
}
