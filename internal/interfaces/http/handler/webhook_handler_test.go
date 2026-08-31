package handler

// Test file conventions in Go:
//   - filename ends in _test.go (excluded from normal builds)
//   - same package name (handler) = a "white-box" test that can see unexported
//     things like verifySignature; use handler_test for black-box tests
//   - functions named TestXxx(t *testing.T) are discovered by `go test`

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chakam/internal/infrastructure/queue"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// sign produces the header value Meta would send for a given body+secret.
func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// newTestEngine wires the handler onto a real gin.Engine. We must go through
// the engine (not call methods via gin.CreateTestContext) because Gin buffers
// the status code and only flushes it during the engine's request lifecycle —
// a direct call leaves the recorder at its default 200. The logger is a no-op
// so tests stay quiet.
type fakeMessageQueue struct {
	processPayloads []queue.ProcessMessagePayload
	processErr      error
}

func (q *fakeMessageQueue) EnqueueProcessMessage(payload queue.ProcessMessagePayload) error {
	q.processPayloads = append(q.processPayloads, payload)
	return q.processErr
}

func (q *fakeMessageQueue) EnqueueSendMessage(queue.SendMessagePayload) error {
	return nil
}

func newTestEngine(secret string) (*gin.Engine, *fakeMessageQueue) {
	gin.SetMode(gin.TestMode)
	queueClient := &fakeMessageQueue{}
	h := NewWebhookHandler(zap.NewNop(), "verify-tok", secret, queueClient, nil, nil)
	r := gin.New()
	r.GET("/webhook/whatsapp", h.VerifyWhatsApp)
	r.POST("/webhook/whatsapp", h.HandleWhatsAppIncoming)
	return r, queueClient
}

const samplePayload = `{"object":"whatsapp_business_account","entry":[{"changes":[{"value":{"messages":[{"from":"234801","id":"wamid.X","type":"text","text":{"body":"hi"}}]}}]}]}`

// Table-driven tests are the idiomatic Go style: one test function, a slice of
// cases, a loop. Easy to add a row without copying boilerplate.
func TestHandleWhatsAppIncoming_Signature(t *testing.T) {
	const secret = "app-secret"
	engine, _ := newTestEngine(secret)

	cases := []struct {
		name       string
		signature  string
		wantStatus int
	}{
		{"valid signature", sign(secret, samplePayload), http.StatusOK},
		{"forged signature", "sha256=deadbeef", http.StatusUnauthorized},
		{"missing header", "", http.StatusUnauthorized},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// httptest gives us a fake ResponseWriter to inspect afterwards.
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", strings.NewReader(samplePayload))
			if tc.signature != "" {
				req.Header.Set("X-Hub-Signature-256", tc.signature)
			}

			engine.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

// When no secret is configured we skip verification (local-dev affordance).
func TestHandleWhatsAppIncoming_NoSecretSkips(t *testing.T) {
	engine, _ := newTestEngine("") // empty secret

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", strings.NewReader(samplePayload))
	// no signature header at all

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (verification should be skipped)", rec.Code, http.StatusOK)
	}
}

func TestHandleWhatsAppIncoming_EnqueuesBeforeAcknowledging(t *testing.T) {
	const secret = "app-secret"
	engine, queueClient := newTestEngine(secret)
	payload := `{"object":"whatsapp_business_account","entry":[{"changes":[{"value":{"messages":[{"from":"234801","id":"wamid.BUTTON","type":"button","button":{"payload":"ACCEPT:req-1","text":"Accept"}}]}}]}]}`

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", strings.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", sign(secret, payload))
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if len(queueClient.processPayloads) != 1 {
		t.Fatalf("enqueued payloads = %d, want 1", len(queueClient.processPayloads))
	}
	got := queueClient.processPayloads[0]
	if got.Button == nil || got.Button.Payload != "ACCEPT:req-1" {
		t.Fatalf("button payload = %#v, want ACCEPT:req-1", got.Button)
	}
}

func TestHandleWhatsAppIncoming_QueueFailureIsRetriable(t *testing.T) {
	const secret = "app-secret"
	engine, queueClient := newTestEngine(secret)
	queueClient.processErr = errors.New("redis unavailable")

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", strings.NewReader(samplePayload))
	req.Header.Set("X-Hub-Signature-256", sign(secret, samplePayload))
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestVerifyWhatsApp_Handshake(t *testing.T) {
	engine, _ := newTestEngine("app-secret") // verifyToken is "verify-tok"

	cases := []struct {
		name       string
		token      string
		wantStatus int
		wantBody   string
	}{
		{"correct token echoes challenge", "verify-tok", http.StatusOK, "chal-123"},
		{"wrong token forbidden", "nope", http.StatusForbidden, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet,
				"/webhook/whatsapp?hub.mode=subscribe&hub.verify_token="+tc.token+"&hub.challenge=chal-123", nil)

			engine.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if rec.Body.String() != tc.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}
