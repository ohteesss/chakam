package router

import "github.com/gin-gonic/gin"

func SetupAdminRoutes(router *gin.Engine) {
	router.GET("/admin/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "OK"})
	})

}
