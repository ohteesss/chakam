package whatsapp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSendText_Success spins up a fake Graph API. We point the client's baseURL
// at it (possible because the test is in the same package), then assert both
// what we SENT (method, path, headers, body) and how we handle the response.
func TestSendText_Success(t *testing.T) {
	var gotAuth, gotPath, gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.TEST123"}]}`))
	}))
	defer server.Close()

	// baseURL points at the fake server — exactly why it's a constructor param
	// and not hardcoded: tests (and different Graph versions) just pass their own.
	c := NewClient("secret-token", "PHONE_ID", server.URL)

	id, err := c.SendText(context.Background(), "234801", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "wamid.TEST123" {
		t.Errorf("message id = %q, want %q", id, "wamid.TEST123")
	}
	if gotAuth != "Bearer secret-token" {
		t.Errorf("auth header = %q, want %q", gotAuth, "Bearer secret-token")
	}
	if gotPath != "/PHONE_ID/messages" {
		t.Errorf("path = %q, want %q", gotPath, "/PHONE_ID/messages")
	}
	// Assert the JSON body we sent has the right shape.
	var sent map[string]any
	if err := json.Unmarshal([]byte(gotBody), &sent); err != nil {
		t.Fatalf("sent body not valid json: %v", err)
	}
	if sent["type"] != "text" || sent["to"] != "234801" {
		t.Errorf("sent body wrong: %s", gotBody)
	}
}

// TestSendText_APIError proves we turn Meta's error envelope into a Go error
// instead of silently succeeding.
func TestSendText_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid phone number","code":100,"fbtrace_id":"AbC"}}`))
	}))
	defer server.Close()

	c := NewClient("tok", "PHONE_ID", server.URL)

	_, err := c.SendText(context.Background(), "bad", "hi")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	// The error should carry Meta's message so logs are useful.
	if !strings.Contains(err.Error(), "Invalid phone number") {
		t.Errorf("error = %q, want it to contain Meta's message", err.Error())
	}
}
