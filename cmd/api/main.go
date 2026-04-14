package main

import (
	"fmt"


	"chakam/internal/pkg/logger"
	"chakam/internal/interfaces/http/router"
	"chakam/internal/config"
)

func main() {
	// 1. Initialize logger
	log := logger.New()
	defer log.Sync()

	log.Info("Starting Chakam API Server...")

	cfg := config.LoadDBConfig()
	fmt.Println("Config:", cfg)

	r := router.SetupRouter()
	router.SetupRoutes(r)
	r.Run(":8080")

}



