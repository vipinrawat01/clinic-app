// Package transport posts pickup coordination details to the clinic's
// transport coordinator over WhatsApp — see the SAMWMC brief's Use Case
// 1, which describes pickup logistics being coordinated inside a
// transport WhatsApp group. Meta's real WhatsApp Business Cloud API has
// no concept of sending to a group at all (group messaging is a
// WhatsApp-Web-automation-only capability, not part of the official
// Business API this platform uses), so this sends a direct message to
// one coordinator's number instead — same real outcome (the coordinator
// sees the pickup details on WhatsApp the moment staff save them), just
// not literally inside a multi-person group thread.
//
// The actual send goes through the main chatbot platform's own
// MessagingHandler.SendWhatsApp endpoint, authenticated with a real
// sk_live_ API key created from that platform's own Developers page —
// the same self-service mechanism any company's external system would
// use to send through its own connected WhatsApp number. This package
// owns only the clinic-specific "what message, to whom, when" decision;
// it never touches WhatsApp credentials directly.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL           string
	APIKey            string
	AgentID           string
	CoordinatorNumber string
	httpClient        *http.Client
}

func New(baseURL, apiKey, agentID, coordinatorNumber string) *Client {
	return &Client{
		BaseURL: baseURL, APIKey: apiKey, AgentID: agentID, CoordinatorNumber: coordinatorNumber,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Configured reports whether every setting SendPickupNotice needs is
// actually present — callers check this first so a clinic that hasn't
// set up transport coordination yet just skips the send (and tells staff
// why) instead of failing on an empty API key or similar.
func (c *Client) Configured() bool {
	return c.BaseURL != "" && c.APIKey != "" && c.AgentID != "" && c.CoordinatorNumber != ""
}

type sendRequest struct {
	AgentID string `json:"agent_id"`
	To      string `json:"to"`
	Message string `json:"message"`
}

// SendPickupNotice posts one appointment's pickup point/time to the
// configured transport coordinator. message is built by the caller
// (internal/http/handlers) so this package stays ignorant of
// appointment/patient shape — it only knows how to deliver a string.
func (c *Client) SendPickupNotice(ctx context.Context, message string) error {
	if !c.Configured() {
		return fmt.Errorf("transport: coordination is not configured (missing main platform API key, agent id, or coordinator number)")
	}

	body, err := json.Marshal(sendRequest{AgentID: c.AgentID, To: c.CoordinatorNumber, Message: message})
	if err != nil {
		return fmt.Errorf("transport: marshal send request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/messaging/whatsapp/send", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("transport: build send request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("transport: send request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("transport: main platform returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
