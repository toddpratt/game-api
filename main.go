package main

import (
	"log"

	"game-api/server"
)

func main() {
	srv := server.NewServer()

	log.Println("Starting game server on :8080")
	if err := srv.Start(":8080"); err != nil {
		log.Fatal(err)
	}
}
