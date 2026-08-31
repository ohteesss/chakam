// Package whatsapp is the OUTBOUND adapter to Meta's WhatsApp Cloud API — the
// code that calls Meta (send a message, create a template, ...). It is the
// mirror of the inbound webhook handler. It lives in the infrastructure layer
// because it adapts our app to an external service.
package whatsapp

import (
	"bytes"
	"chakam/internal/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Client talks to the Cloud API. Create ONE and share it: it holds a pooled
// *http.Client, and the fields are its dependencies (same injection idea as the
// handler).
type Client struct {
	http          *http.Client
	baseURL       string // e.g. https://graph.facebook.com/v21.0 — from config, not hardcoded
	accessToken   string // "Bearer" token from your Meta app (System User token)
	phoneNumberID string // the sender phone number's ID, not the phone number
}

type templateParameter struct {
	Type          string `json:"type"`                     // body: "text" — button: "payload"
	ParameterName string `json:"parameter_name,omitempty"` // body named params only
	Text          string `json:"text,omitempty"`           // value for "text" params
	Payload       string `json:"payload,omitempty"`        // value for "payload" (quick-reply) params
}
type templateComponent struct {
	Type       string              `json:"type"`
	SubType    string              `json:"sub_type,omitempty"`
	Index      string              `json:"index,omitempty"`
	Parameters []templateParameter `json:"parameters"`
}

type templatePayload struct {
	MessagingProduct string `json:"messaging_product"`
	To               string `json:"to"`
	Type             string `json:"type"`
	Template         struct {
		Name     string `json:"name"`
		Language struct {
			Code string `json:"code"`
		} `json:"language"`
		Components []templateComponent `json:"components"`
	} `json:"template"`
}

type sendTemplatePayload struct {
	MessagingProduct string               `json:"messaging_product"`
	To               string               `json:"to"`
	Type             string               `json:"type"`
	Template         innerTemplatePayload `json:"template"`
}

type innerTemplatePayload struct {
	Name     string `json:"name"`
	Language struct {
		Code string `json:"code"`
	} `json:"language"`
	Components []templateComponent `json:"components"`
}

type sendTextPayload struct {
	MessagingProduct string `json:"messaging_product"`
	To               string `json:"to"`
	Type             string `json:"type"`
	Text             struct {
		Body string `json:"body"`
	} `json:"text"`
}
type sendResponse struct {
	Messages []struct {
		ID string `json:"id"` // the "wamid..." id of the sent message
	} `json:"messages"`
}

type apiErrorEnvelope struct {
	Error struct {
		Message   string `json:"message"`
		Code      int    `json:"code"`
		FBTraceID string `json:"fbtrace_id"`
	} `json:"error"`
}

// NewClient is the constructor. All three values come from config
// (WHATSAPP_ACCESS_TOKEN / WHATSAPP_PHONE_NUMBER_ID / WHATSAPP_BASE_URL), so the
// Graph API version is configurable and nothing is baked into the binary.
func NewClient(accessToken, phoneNumberID, baseURL string) *Client {
	return &Client{
		// Reuse ONE http.Client across all calls — it pools TCP connections.
		// Making a fresh client per request defeats the pool and leaks sockets.
		// ALWAYS set a Timeout: the zero-value client waits forever, so one slow
		// Meta response could hang your whole handler.
		http:          &http.Client{Timeout: 10 * time.Second},
		baseURL:       baseURL,
		accessToken:   accessToken,
		phoneNumberID: phoneNumberID,
	}
}

func (c *Client) post(ctx context.Context, path string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("whatsapp: encode request: %w", err)
	}

	url := c.baseURL + "/" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("whatsapp: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var env apiErrorEnvelope
		if json.Unmarshal(respBody, &env) == nil && env.Error.Message != "" {
			return fmt.Errorf("whatsapp: api error code %d: %s (trace %s)",
				env.Error.Code, env.Error.Message, env.Error.FBTraceID)
		}
		return fmt.Errorf("whatsapp: unexpected status %d: %s", resp.StatusCode, respBody)
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("whatsapp: decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) SendText(ctx context.Context, to, body string) (string, error) {
	payload := sendTextPayload{
		MessagingProduct: "whatsapp",
		To:               to,
		Type:             "text",
	}
	payload.Text.Body = body

	var res sendResponse
	if err := c.post(ctx, c.phoneNumberID+"/messages", payload, &res); err != nil {
		return "", err
	}
	if len(res.Messages) == 0 {
		return "", fmt.Errorf("whatsapp: send succeeded but no message id returned")
	}
	return res.Messages[0].ID, nil
}

func (c *Client) SendTemplate(ctx context.Context, payload sendTemplatePayload) (string, error) {
	p := templatePayload{
		MessagingProduct: "whatsapp",
		To:               payload.To,
		Type:             "template",
	}
	p.Template.Name = payload.Template.Name
	p.Template.Language.Code = payload.Template.Language.Code
	p.Template.Components = payload.Template.Components

	var res sendResponse
	if err := c.post(ctx, c.phoneNumberID+"/messages", payload, &res); err != nil {
		return "", err
	}
	if len(res.Messages) == 0 {
		return "", fmt.Errorf("whatsapp: send succeeded but no message id returned")
	}
	return res.Messages[0].ID, nil
}

func (c *Client) SendTemplateHello(ctx context.Context, to string) (string, error) {
	log := logger.New()
	log.Info("sending template message", zap.String("to", logger.MaskPhone(to)))

	payl := sendTemplatePayload{
		MessagingProduct: "whatsapp",
		To:               to,
		Type:             "template",
		Template: innerTemplatePayload{
			Name: "chakkam_confrim_request",
			Language: struct {
				Code string `json:"code"`
			}{
				Code: "en",
			},
			Components: []templateComponent{
				{
					Type: "body",
					Parameters: []templateParameter{
						{
							Type:          "text",
							ParameterName: "number",
							Text:          "+2349035177909",
						},
						{
							Type:          "text",
							ParameterName: "description",
							Text:          "do you agree to the agreement?",
						},
					},
				},
				{
					Type:    "button",
					SubType: "quick_reply",
					Index:   "0",
					Parameters: []templateParameter{
						{Type: "payload", Payload: "ACCEPT"},
					},
				},
				// Quick-reply button at position 1 (note the different index).
				{
					Type:    "button",
					SubType: "quick_reply",
					Index:   "1",
					Parameters: []templateParameter{
						{Type: "payload", Payload: "REJECT"},
					},
				},
			},
		},
	}
	var res sendResponse
	if err := c.post(ctx, c.phoneNumberID+"/messages", payl, &res); err != nil {
		log.Error("failed to send template message", zap.Error(err))
		return "", err
	}
	log.Info("template message sent", zap.String("message_id", firstMessageID(res)))
	if len(res.Messages) == 0 {
		return "", fmt.Errorf("whatsapp: s	end succeeded but no message id returned")
	}
	return res.Messages[0].ID, nil
}
func (c *Client) SendTemplateConfirmAgreement(ctx context.Context, to string, from string, description string, agreementID string) (string, error) {
	log := logger.New()
	log.Info("sending template message", zap.String("to", logger.MaskPhone(to)))

	payl := sendTemplatePayload{
		MessagingProduct: "whatsapp",
		To:               to,
		Type:             "template",
		Template: innerTemplatePayload{
			Name: "chakkam_confrim_request",
			Language: struct {
				Code string `json:"code"`
			}{
				Code: "en",
			},
			Components: []templateComponent{
				{
					Type: "body",
					Parameters: []templateParameter{
						{
							Type:          "text",
							ParameterName: "number",
							Text:          from,
						},
						{
							Type:          "text",
							ParameterName: "description",
							Text:          description,
						},
					},
				},
				{
					Type:    "button",
					SubType: "quick_reply",
					Index:   "0",
					Parameters: []templateParameter{
						{Type: "payload", Payload: "ACCEPT:" + agreementID},
					},
				},
				{
					Type:    "button",
					SubType: "quick_reply",
					Index:   "1",
					Parameters: []templateParameter{
						{Type: "payload", Payload: "REJECT:" + agreementID},
					},
				},
			},
		},
	}
	var res sendResponse
	if err := c.post(ctx, c.phoneNumberID+"/messages", payl, &res); err != nil {
		log.Error("failed to send template message", zap.Error(err))
		return "", err
	}
	log.Info("template message sent", zap.String("message_id", firstMessageID(res)))
	if len(res.Messages) == 0 {
		return "", fmt.Errorf("whatsapp: s	end succeeded but no message id returned")
	}
	return res.Messages[0].ID, nil
}

func firstMessageID(response sendResponse) string {
	if len(response.Messages) == 0 {
		return ""
	}
	return response.Messages[0].ID
}
