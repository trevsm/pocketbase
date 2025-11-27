package mailer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/security"
)

var _ Mailer = (*ResendClient)(nil)

const resendAPIEndpoint = "https://api.resend.com/emails"

// ResendClient defines a Resend mail client structure that implements
// `mailer.Mailer` interface.
type ResendClient struct {
	onSend *hook.Hook[*SendEvent]

	// APIKey is the Resend API key for authentication.
	APIKey string
}

// OnSend implements [mailer.SendInterceptor] interface.
func (c *ResendClient) OnSend() *hook.Hook[*SendEvent] {
	if c.onSend == nil {
		c.onSend = &hook.Hook[*SendEvent]{}
	}
	return c.onSend
}

// Send implements [mailer.Mailer] interface.
func (c *ResendClient) Send(m *Message) error {
	if c.onSend != nil {
		return c.onSend.Trigger(&SendEvent{Message: m}, func(e *SendEvent) error {
			return c.send(e.Message)
		})
	}

	return c.send(m)
}

// resendAttachment represents an attachment in the Resend API payload.
type resendAttachment struct {
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	ContentType string `json:"content_type,omitempty"`
}

// resendPayload represents the JSON payload for the Resend API.
type resendPayload struct {
	From        string             `json:"from"`
	To          []string           `json:"to"`
	Cc          []string           `json:"cc,omitempty"`
	Bcc         []string           `json:"bcc,omitempty"`
	Subject     string             `json:"subject"`
	HTML        string             `json:"html,omitempty"`
	Text        string             `json:"text,omitempty"`
	Headers     map[string]string  `json:"headers,omitempty"`
	Attachments []resendAttachment `json:"attachments,omitempty"`
}

// resendErrorResponse represents an error response from the Resend API.
type resendErrorResponse struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Name       string `json:"name"`
}

func (c *ResendClient) send(m *Message) error {
	if c.APIKey == "" {
		return errors.New("resend API key is required")
	}

	// Build the payload
	payload := resendPayload{
		From:    m.From.String(),
		To:      addressesToStrings(m.To, true),
		Subject: m.Subject,
		HTML:    m.HTML,
	}

	// Set text content
	if m.Text == "" {
		// Try to generate a plain text version of the HTML
		if plain, err := html2Text(m.HTML); err == nil {
			payload.Text = plain
		}
	} else {
		payload.Text = m.Text
	}

	// Add CC recipients
	if len(m.Cc) > 0 {
		payload.Cc = addressesToStrings(m.Cc, true)
	}

	// Add BCC recipients
	if len(m.Bcc) > 0 {
		payload.Bcc = addressesToStrings(m.Bcc, true)
	}

	// Add custom headers
	if len(m.Headers) > 0 {
		payload.Headers = make(map[string]string)
		var hasMessageId bool
		for k, v := range m.Headers {
			if strings.EqualFold(k, "Message-ID") {
				hasMessageId = true
			}
			payload.Headers[k] = v
		}
		// Add a default message id if missing
		if !hasMessageId {
			fromParts := strings.Split(m.From.Address, "@")
			if len(fromParts) == 2 {
				payload.Headers["Message-ID"] = fmt.Sprintf("<%s@%s>",
					security.PseudorandomString(15),
					fromParts[1],
				)
			}
		}
	}

	// Process regular attachments
	if len(m.Attachments) > 0 {
		for name, data := range m.Attachments {
			attachment, err := c.prepareAttachment(name, data)
			if err != nil {
				return fmt.Errorf("failed to prepare attachment %s: %w", name, err)
			}
			payload.Attachments = append(payload.Attachments, attachment)
		}
	}

	// Process inline attachments (Resend treats them the same as regular attachments)
	if len(m.InlineAttachments) > 0 {
		for name, data := range m.InlineAttachments {
			attachment, err := c.prepareAttachment(name, data)
			if err != nil {
				return fmt.Errorf("failed to prepare inline attachment %s: %w", name, err)
			}
			payload.Attachments = append(payload.Attachments, attachment)
		}
	}

	// Marshal payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal resend payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest(http.MethodPost, resendAPIEndpoint, bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create resend request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send resend request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		var errResp resendErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
			return fmt.Errorf("resend API error (%d): %s", resp.StatusCode, errResp.Message)
		}
		return fmt.Errorf("resend API error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// prepareAttachment reads the attachment data and converts it to a resendAttachment.
func (c *ResendClient) prepareAttachment(name string, data io.Reader) (resendAttachment, error) {
	// Detect MIME type
	r, mimeType, err := detectReaderMimeType(data)
	if err != nil {
		return resendAttachment{}, err
	}

	// Read all content
	content, err := io.ReadAll(r)
	if err != nil {
		return resendAttachment{}, err
	}

	// Encode to base64
	encoded := base64.StdEncoding.EncodeToString(content)

	return resendAttachment{
		Filename:    name,
		Content:     encoded,
		ContentType: mimeType,
	}, nil
}

