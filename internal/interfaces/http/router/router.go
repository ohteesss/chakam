package router

import (
	"chakam/internal/interfaces/http/handler"
	httpmiddleware "chakam/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func SetupRouter(log *zap.Logger, maxBodyBytes int64) *gin.Engine {
	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	router.Use(
		httpmiddleware.RequestID(),
		httpmiddleware.SecurityHeaders(),
		httpmiddleware.BodyLimit(maxBodyBytes),
		httpmiddleware.AccessLog(log),
		httpmiddleware.Recovery(log),
	)
	return router
}

func SetupRoutes(router *gin.Engine, webhook *handler.WebhookHandler, api *handler.ApiHandler, health *handler.HealthHandler, internalAPIKey string) {
	router.GET("/health", health.Live)
	router.GET("/livez", health.Live)
	router.GET("/readyz", health.Ready)

	router.GET("/webhook/whatsapp", webhook.VerifyWhatsApp)
	router.POST("/webhook/whatsapp", webhook.HandleWhatsAppIncoming)
	internal := router.Group("/")
	internal.Use(httpmiddleware.APIKey(internalAPIKey))
	internal.POST("/send-message", webhook.SendMessage)
	internal.GET("/api/v1/agreements", api.GetAgreements)
	internal.GET("/api/v1/agreements/:id", api.GetAgreementById)
	internal.GET("/api/v1/users", api.GetUsers)
}
