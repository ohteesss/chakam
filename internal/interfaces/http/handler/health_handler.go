package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type databasePinger interface {
	PingContext(ctx context.Context) error
}

type queuePinger interface {
	Ping() error
}

type HealthHandler struct {
	database databasePinger
	queue    queuePinger
}

func NewHealthHandler(database databasePinger, queue queuePinger) *HealthHandler {
	return &HealthHandler{database: database, queue: queue}
}

func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	checks := gin.H{"database": "ok", "queue": "ok"}
	ready := true
	if h.database == nil || h.database.PingContext(ctx) != nil {
		checks["database"] = "unavailable"
		ready = false
	}
	if h.queue == nil || h.queue.Ping() != nil {
		checks["queue"] = "unavailable"
		ready = false
	}
	if !ready {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "checks": checks})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready", "checks": checks})
}
