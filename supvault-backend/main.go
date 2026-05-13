package main

import (
	"log"

	"github.com/supvault/supvault-backend/cmd/server"
)

func main() {
	if err := server.Run(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
