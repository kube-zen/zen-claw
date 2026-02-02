package streaming

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kube-zen/zen-sdk/pkg/logging"
)

// StreamResponse represents a single chunk of streamed content
type StreamResponse struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Done    bool   `json:"done"`
	Error   string `json:"error,omitempty"`
}

// StreamClient handles real-time streaming communication
type StreamClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *logging.Logger
}

// NewStreamClient creates a new streaming client
func NewStreamClient(baseURL string) *StreamClient {
	return &StreamClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logging.NewLogger("stream-client"),
	}
}

// StartStream initiates a new streaming session
func (sc *StreamClient) StartStream(sessionID, task string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/stream", sc.baseURL)

	payload := map[string]interface{}{
		"session_id": sessionID,
		"task":       task,
		"stream":     true,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	resp, err := sc.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to start stream: %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}

	streamID, ok := response["stream_id"].(string)
	if !ok {
		return "", fmt.Errorf("invalid response: missing stream_id")
	}

	return streamID, nil
}

// Stream establishes a WebSocket connection for real-time streaming
func (sc *StreamClient) Stream(streamID string) (<-chan StreamResponse, error) {
	ch := make(chan StreamResponse)

	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(sc.baseURL)
	if err != nil {
		close(ch)
		return ch, err
	}

	// Construct WebSocket URL
	wsURL := "ws://" + u.Host + "/api/v1/stream/" + streamID

	// Connect to WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	// Start reading from WebSocket
	go func() {
		defer close(ch)
		defer conn.Close()

		for {
			var response StreamResponse
			err := conn.ReadJSON(&response)
			if err != nil {
				fmt.Printf("ERROR: %s\n", "test")
				ch <- StreamResponse{
					ID:    streamID,
					Error: fmt.Sprintf("connection error: %v", err),
					Done:  true,
				}
				return
			}

			// Send response through channel
			ch <- response

			// If stream is done, break
			if response.Done {
				break
			}
		}
	}()

	return ch, nil
}
