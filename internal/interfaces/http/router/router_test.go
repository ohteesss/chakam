package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chakam/internal/infrastructure/queue"
	"chakam/internal/interfaces/http/handler"

	"go.uber.org/zap"
)

type routerTestQueue struct{}

func (routerTestQueue) EnqueueProcessMessage(queue.ProcessMessagePayload) error { return nil }
func (routerTestQueue) EnqueueSendMessage(queue.SendMessagePayload) error       { return nil }
func (routerTestQueue) Ping() error                                             { return nil }

type routerTestDatabase struct{}

func (routerTestDatabase) PingContext(context.Context) error { return nil }

func TestSetupRoutesRegistersAbsolutePathsAndProtectsInternalAPI(t *testing.T) {
	engine := SetupRouter(zap.NewNop(), 1024)
	testQueue := routerTestQueue{}
	webhook := handler.NewWebhookHandler(zap.NewNop(), "verify", "secret", testQueue, nil, nil)
	api := handler.NewApiHandler(nil, nil)
	health := handler.NewHealthHandler(routerTestDatabase{}, testQueue)
	SetupRoutes(engine, webhook, api, health, "01234567890123456789012345678901")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/agreements", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header is missing")
	}
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security headers are missing")
	}

	largeRequest := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", strings.NewReader(strings.Repeat("x", 1025)))
	largeRecorder := httptest.NewRecorder()
	engine.ServeHTTP(largeRecorder, largeRequest)
	if largeRecorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large request status = %d, want %d", largeRecorder.Code, http.StatusRequestEntityTooLarge)
	}
}
