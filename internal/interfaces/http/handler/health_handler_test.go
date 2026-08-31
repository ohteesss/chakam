package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeDatabasePinger struct{ err error }

func (f fakeDatabasePinger) PingContext(context.Context) error { return f.err }

type fakeQueuePinger struct{ err error }

func (f fakeQueuePinger) Ping() error { return f.err }

func TestHealthReady(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		database   error
		queue      error
		wantStatus int
	}{
		{name: "ready", wantStatus: http.StatusOK},
		{name: "database unavailable", database: errors.New("down"), wantStatus: http.StatusServiceUnavailable},
		{name: "queue unavailable", queue: errors.New("down"), wantStatus: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			h := NewHealthHandler(fakeDatabasePinger{test.database}, fakeQueuePinger{test.queue})
			router.GET("/readyz", h.Ready)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
		})
	}
}
