package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/peerblade/PeerBlade/agent/internal/wireguard"
)

const maxResponseSize = 1 << 20

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type AgentResponse struct {
	ID             string   `json:"id"`
	ServerID       string   `json:"serverId"`
	Status         string   `json:"status"`
	Version        string   `json:"version"`
	Capabilities   []string `json:"capabilities"`
	RegisteredAt   string   `json:"registeredAt"`
	LastSeenAt     string   `json:"lastSeenAt"`
	LastSnapshotAt *string  `json:"lastSnapshotAt"`
}

type AgentCommand struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	InterfaceName string   `json:"interfaceName"`
	PublicKey     string   `json:"publicKey"`
	Name          string   `json:"name"`
	AllowedIPs    []string `json:"allowedIps"`
}

type AgentCommandResult struct {
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
	Payload string `json:"payload,omitempty"`
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Control Plane returned HTTP %d: %s", e.StatusCode, e.Message)
}

func NewClient(baseURL, token string, httpClient *http.Client) (*Client, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return nil, errors.New("PEERBLADE_API_URL must be an absolute HTTP or HTTPS URL")
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("PEERBLADE_AGENT_TOKEN is required")
	}
	if httpClient == nil {
		return nil, errors.New("HTTP client is required")
	}

	return &Client{
		baseURL:    strings.TrimRight(parsedURL.String(), "/"),
		token:      token,
		httpClient: httpClient,
	}, nil
}

func (c *Client) Register(
	ctx context.Context,
	serverID string,
	version string,
	capabilities []string,
) (AgentResponse, error) {
	return c.post(ctx, "/api/agent/v1/register", map[string]interface{}{
		"serverId":     serverID,
		"version":      version,
		"capabilities": capabilities,
	})
}

func (c *Client) Heartbeat(ctx context.Context, version string) (AgentResponse, error) {
	return c.post(ctx, "/api/agent/v1/heartbeat", map[string]string{
		"version": version,
	})
}

func (c *Client) Snapshot(ctx context.Context, snapshot wireguard.Snapshot) (AgentResponse, error) {
	return c.post(ctx, "/api/agent/v1/snapshot", snapshot)
}

func (c *Client) NextCommand(ctx context.Context) (*AgentCommand, error) {
	var response struct {
		Command *AgentCommand `json:"command"`
	}
	if err := c.request(ctx, http.MethodGet, "/api/agent/v1/commands/next", nil, &response); err != nil {
		return nil, err
	}

	return response.Command, nil
}

func (c *Client) CompleteCommand(
	ctx context.Context,
	commandID string,
	result AgentCommandResult,
) error {
	return c.request(
		ctx,
		http.MethodPost,
		"/api/agent/v1/commands/"+url.PathEscape(commandID)+"/result",
		result,
		nil,
	)
}

func (c *Client) post(ctx context.Context, path string, payload interface{}) (AgentResponse, error) {
	var agent AgentResponse
	if err := c.request(ctx, http.MethodPost, path, payload, &agent); err != nil {
		return AgentResponse{}, err
	}

	return agent, nil
}

func (c *Client) request(
	ctx context.Context,
	method string,
	path string,
	payload interface{},
	result interface{},
) error {
	var body io.Reader

	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		method,
		c.baseURL+path,
		body,
	)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return decodeAPIError(response.StatusCode, responseBody)
	}

	if result == nil {
		return nil
	}
	if err := json.Unmarshal(responseBody, result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func decodeAPIError(statusCode int, body []byte) error {
	var response struct {
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(body, &response); err == nil {
		var message string
		if json.Unmarshal(response.Message, &message) == nil && message != "" {
			return &APIError{StatusCode: statusCode, Message: message}
		}

		var messages []string
		if json.Unmarshal(response.Message, &messages) == nil && len(messages) > 0 {
			return &APIError{StatusCode: statusCode, Message: strings.Join(messages, ". ")}
		}
	}

	return &APIError{StatusCode: statusCode, Message: http.StatusText(statusCode)}
}
