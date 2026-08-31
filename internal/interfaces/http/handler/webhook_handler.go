package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"chakam/internal/application/agreement"
	"chakam/internal/application/user"
	"chakam/internal/infrastructure/queue"
	"chakam/internal/pkg/logger"
	"chakam/internal/pkg/validator"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type WebhookHandler struct {
	log              *zap.Logger
	verifyToken      string // GET handshake (the token you chose in the dashboard)
	appSecret        string // POST signature key (App Secret from the Meta app)
	queueClient      messageQueue
	agreementService *agreement.Service
	userService      *user.Service
}

type messageQueue interface {
	EnqueueProcessMessage(payload queue.ProcessMessagePayload) error
	EnqueueSendMessage(payload queue.SendMessagePayload) error
}

func NewWebhookHandler(log *zap.Logger, verifyToken, appSecret string, queueClient messageQueue, agreementService *agreement.Service, userService *user.Service) *WebhookHandler {
	return &WebhookHandler{log: log, verifyToken: verifyToken, appSecret: appSecret, queueClient: queueClient, agreementService: agreementService, userService: userService}
}

type whatsAppEvent struct {
	Object string          `json:"object"`
	Entry  []whatsAppEntry `json:"entry"`
}

type whatsAppEntry struct {
	ID      string           `json:"id"`
	Changes []whatsAppChange `json:"changes"`
}

type whatsAppChange struct {
	Field string        `json:"field"`
	Value whatsAppValue `json:"value"`
}

type whatsAppContact struct {
	WaID    string `json:"wa_id"` // the sender's WhatsApp id
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
}

type whatsAppValue struct {
	MessagingProduct string `json:"messaging_product"`
	// dropping messages — so it must be a slice.
	Contacts []whatsAppContact `json:"contacts"`
	Messages []whatsAppMessage `json:"messages"`

	Statuses []whatsAppStatus `json:"statuses"`

}

type whatsAppMessage struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"` // "text", "image", "audio", ...
	Text      struct {
		Body string `json:"body"`
	} `json:"text"`
	Button struct {
		Payload string `json:"payload"`
		Text    string `json:"text"`
	} `json:"button"`
	Context struct {
		From string `json:"from"`
		ID   string `json:"id"`
	} `json:"context"`
}

type whatsAppStatus struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// --- helpers ----------------------------------------------------------------

func (h *WebhookHandler) logWhatsAppMessage(msg whatsAppMessage) {
	h.log.Info("received whatsapp message",
		zap.String("from", logger.MaskPhone(msg.From)),
		zap.String("type", msg.Type),
		zap.String("message_id", msg.ID),
		zap.Int("body_length", len(msg.Text.Body)),
		zap.String("timestamp", msg.Timestamp),
	)
}

func (h *WebhookHandler) logWhatsAppContact(contact whatsAppContact) {
	h.log.Info("received whatsapp contact",
		zap.String("from", logger.MaskPhone(contact.WaID)),
	)
}

func (h *WebhookHandler) logWhatsAppStatus(status whatsAppStatus) {
	h.log.Info("received whatsapp status",
		zap.String("id", status.ID),
		zap.String("status", status.Status),
		zap.String("timestamp", status.Timestamp),
	)
}

func (h *WebhookHandler) verifySignature(header string, body []byte) bool {
	if h.appSecret == "" {
		h.log.Warn("WHATSAPP_APP_SECRET not set; skipping signature verification")
		return true
	}

	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	want := strings.TrimPrefix(header, prefix)

	mac := hmac.New(sha256.New, []byte(h.appSecret))
	mac.Write(body)
	got := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(want), []byte(got))
}

func (h *WebhookHandler) VerifyWhatsApp(c *gin.Context) {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	if mode == "subscribe" && token == h.verifyToken {
		h.log.Info("whatsapp webhook verified")
		c.String(http.StatusOK, challenge)
		return
	}

	h.log.Warn("whatsapp webhook verification failed", zap.String("mode", mode))
	c.Status(http.StatusForbidden)
}


func (h *WebhookHandler) HandleWhatsAppIncoming(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		h.log.Warn("could not read whatsapp webhook body", zap.Error(err))
		c.Status(http.StatusOK)
		return
	}

	if !h.verifySignature(c.GetHeader("X-Hub-Signature-256"), body) {
		h.log.Warn("rejected whatsapp webhook with invalid signature")
		c.Status(http.StatusUnauthorized)
		return
	}
	var event whatsAppEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.log.Warn("could not parse whatsapp webhook", zap.Error(err))
		c.Status(http.StatusOK)
		return
	}

	if err := h.processEvent(event); err != nil {
		h.log.Error("could not enqueue whatsapp event", zap.Error(err))
		c.Status(http.StatusServiceUnavailable)
		return
	}

	c.Status(http.StatusOK)
}

func (h *WebhookHandler) processEvent(event whatsAppEvent) error {
	var name *string
	var payload queue.ProcessMessagePayload
	for _, entry := range event.Entry {
		for _, change := range entry.Changes {
			for _, contact := range change.Value.Contacts {
				h.logWhatsAppContact(contact)
				if contact.Profile.Name != "" {
					name = &contact.Profile.Name
				}

			}
			for _, msg := range change.Value.Messages {
				h.logWhatsAppMessage(msg)

				payload = queue.ProcessMessagePayload{
					From:      msg.From,
					Name:      name,
					Button:    &msg.Button,
					Body:      msg.Text.Body,
					Type:      msg.Type,
					MessageID: &msg.ID,
				}

				if err := h.queueClient.EnqueueProcessMessage(payload); err != nil {
					return err
				}

				h.log.Info("enqueued process message", zap.String("message_id", msg.ID), zap.String("type", msg.Type))
			}
			for _, status := range change.Value.Statuses {
				h.logWhatsAppStatus(status)
			}
		}
	}
	return nil
}

func (h *WebhookHandler) SendMessage(c *gin.Context) {
	var payload struct {
		To      string `json:"to"`
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.queueClient.EnqueueSendMessage(queue.SendMessagePayload{
		To:      payload.To,
		Message: payload.Message,
	})
	if err != nil {
		h.log.Error("failed to enqueue send message", zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "message queue unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "message enqueued"})

	h.log.Info("message enqueued", zap.String("to", logger.MaskPhone(payload.To)))
}

func (h *WebhookHandler) GetMyAgreements(c *gin.Context) {
	var payload struct {
		From string `json:"from"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}


	fromWaID, err := validator.FromWaID(payload.From)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}


	u, err := h.userService.CreateUser(c.Request.Context(), fromWaID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.log.Info("agreement lookup", zap.String("user_id", u.PublicID))
	agreements, err := h.agreementService.GetAgreements(c.Request.Context(), "all", u.PublicID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"agreements": agreements})
}
