package main

import (
	"log"

	"github.com/HoBom-s/hobom-event-processor/internal/infra/http"
)

func main() {
	log.Println("Starting Outbox Processor...")

	http.StartHTTPServer(8080)
}