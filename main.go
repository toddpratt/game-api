package main

import (
	"log"

	"game-api/config"
	"game-api/server"
)

func main() {
	cfg := config.Load()
	srv := server.NewServer(cfg)

	addr := ":" + cfg.Port
	log.Printf("Starting game server on %s", addr)
	log.Printf("CORS allowed origins: %s", cfg.AllowedOrigins)

	if err := srv.Start(addr); err != nil {
		log.Fatal(err)
	}
}
