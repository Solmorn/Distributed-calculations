package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Solmorn/Distributed-calculations/internal/agent"
	"github.com/Solmorn/Distributed-calculations/internal/db"
)

func main() {
	database := db.GetInstance()
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("Error close bd: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Dead signal has been received, close bd...")
		if err := database.Close(); err != nil {
			log.Printf("Error close bd: %v", err)
		}
		os.Exit(0)
	}()

	go agent.StartAgent()

	orchestrator.RunOrchestrator()

}
