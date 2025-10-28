package waha

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the HTTP client for WAHA API
type Client struct {
	BaseURL string
	APIKey  string
	http    *http.Client
}

// NewClient creates a new WAHA API client
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateSessionRequest represents the request to create a WAHA session
type CreateSessionRequest struct {
	Name   string         `json:"name"`
	Start  bool           `json:"start,omitempty"`
	Config *SessionConfig `json:"config,omitempty"`
}

// SessionConfig represents the configuration for a WAHA session
type SessionConfig struct {
	Metadata map[string]string `json:"metadata,omitempty"`
	Webhooks []WebhookConfig   `json:"webhooks,omitempty"`
}

// WebhookConfig represents webhook configuration
type WebhookConfig struct {
	URL           string         `json:"url"`
	Events        []string       `json:"events"`
	CustomHeaders []CustomHeader `json:"customHeaders,omitempty"`
	HMAC          *HMACConfig    `json:"hmac,omitempty"`
	Retries       *RetryConfig   `json:"retries,omitempty"`
}

// CustomHeader represents a custom HTTP header
type CustomHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HMACConfig represents HMAC authentication configuration
type HMACConfig struct {
	Key string `json:"key"`
}

// RetryConfig represents retry policy configuration
type RetryConfig struct {
	Policy       string `json:"policy"` // constant, linear, exponential
	Attempts     int    `json:"attempts"`
	DelaySeconds int    `json:"delaySeconds"`
}

// SessionResponse represents the response from WAHA session API
type SessionResponse struct {
	Name   string                 `json:"name"`
	Status string                 `json:"status"`
	Config map[string]interface{} `json:"config,omitempty"`
	Me     *SessionMeInfo         `json:"me,omitempty"`
}

// SessionMeInfo represents the authenticated WhatsApp account info
type SessionMeInfo struct {
	ID       string `json:"id"`
	PushName string `json:"pushName"`
}

// PairingCodeRequest represents the request to get a pairing code
type PairingCodeRequest struct {
	PhoneNumber string `json:"phoneNumber"`
}

// PairingCodeResponse represents the response with pairing code
type PairingCodeResponse struct {
	Code string `json:"code"`
}

// UpdateSession updates an existing WAHA session
func (c *Client) UpdateSession(sessionName string, req CreateSessionRequest) (*SessionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	fmt.Printf("[WAHA Client] Updating session request body: %s\n", string(body))

	httpReq, err := http.NewRequest("PUT", fmt.Sprintf("%s/api/sessions/%s", c.BaseURL, sessionName), bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("WAHA API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Printf("[WAHA Client] Session updated successfully: %+v\n", result)

	return &result, nil
}

// CreateSession creates a new WAHA session
func (c *Client) CreateSession(req CreateSessionRequest) (*SessionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Log request body for debugging
	fmt.Printf("[WAHA Client] Creating session request body: %s\n", string(body))

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/api/sessions", c.BaseURL), bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("WAHA API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Printf("[WAHA Client] Session created successfully: %+v\n", result)

	return &result, nil
}

// GetSession retrieves session information
func (c *Client) GetSession(sessionName string) (*SessionResponse, error) {
	httpReq, err := http.NewRequest("GET", fmt.Sprintf("%s/api/sessions/%s", c.BaseURL, sessionName), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("WAHA API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// RequestPairingCode requests a pairing code for phone number authentication
func (c *Client) RequestPairingCode(sessionName, phoneNumber string) (*PairingCodeResponse, error) {
	reqBody := PairingCodeRequest{
		PhoneNumber: phoneNumber,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/api/%s/auth/request-code", c.BaseURL, sessionName), bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("WAHA API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result PairingCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// StopSession stops a WAHA session
func (c *Client) StopSession(sessionName string) error {
	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/api/sessions/%s/stop", c.BaseURL, sessionName), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("WAHA API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// RestartSession restarts a WAHA session
func (c *Client) RestartSession(sessionName string) error {
	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/api/sessions/%s/restart", c.BaseURL, sessionName), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("WAHA API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// LogoutSession logs out and deletes a WAHA session
func (c *Client) LogoutSession(sessionName string) error {
	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/api/sessions/%s/logout", c.BaseURL, sessionName), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("WAHA API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeleteSession deletes a WAHA session
func (c *Client) DeleteSession(sessionName string) error {
	httpReq, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/sessions/%s", c.BaseURL, sessionName), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("WAHA API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GroupInfo represents information about a WhatsApp group (matches WAHA API response)
type GroupInfo struct {
	JID                           string              `json:"JID"`
	Name                          string              `json:"Name"`
	NameSetAt                     string              `json:"NameSetAt,omitempty"`
	NameSetBy                     string              `json:"NameSetBy,omitempty"`
	Topic                         string              `json:"Topic,omitempty"`
	TopicSetAt                    string              `json:"TopicSetAt,omitempty"`
	GroupCreated                  string              `json:"GroupCreated,omitempty"`
	OwnerJID                      string              `json:"OwnerJID,omitempty"`
	Participants                  []GroupParticipant  `json:"Participants,omitempty"`
	IsAnnounce                    bool                `json:"IsAnnounce,omitempty"`
	IsLocked                      bool                `json:"IsLocked,omitempty"`
	IsEphemeral                   bool                `json:"IsEphemeral,omitempty"`
	ParticipantVersionID          string              `json:"ParticipantVersionID,omitempty"`
}

// GroupParticipant represents a participant in a WhatsApp group
type GroupParticipant struct {
	JID          string `json:"JID"`
	PhoneNumber  string `json:"PhoneNumber"`
	LID          string `json:"LID"`
	IsAdmin      bool   `json:"IsAdmin"`
	IsSuperAdmin bool   `json:"IsSuperAdmin"`
	DisplayName  string `json:"DisplayName"`
}

// GetGroup retrieves information about a WhatsApp group
func (c *Client) GetGroup(sessionName, groupID string) (*GroupInfo, error) {
	httpReq, err := http.NewRequest("GET", fmt.Sprintf("%s/api/%s/groups/%s", c.BaseURL, sessionName, groupID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("WAHA API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result GroupInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ContactInfo represents information about a WhatsApp contact (matches WAHA API response)
type ContactInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	NotifyName  string `json:"notifyName"`
	PushName    string `json:"pushname"`
	ShortName   string `json:"shortName"`
	Number      string `json:"number"`
	IsMe        bool   `json:"isMe"`
	IsUser      bool   `json:"isUser"`
	IsGroup     bool   `json:"isGroup"`
	IsWAContact bool   `json:"isWAContact"`
	IsMyContact bool   `json:"isMyContact"`
}

// GetContact retrieves information about a WhatsApp contact
func (c *Client) GetContact(sessionName, contactID string) (*ContactInfo, error) {
	// Build URL with query parameters
	url := fmt.Sprintf("%s/api/contacts?session=%s&contactId=%s", c.BaseURL, sessionName, contactID)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("WAHA API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result ContactInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
