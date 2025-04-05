package ws

import (
	"log"
	"net/http"
	"time"
)

func sendWithRetry(ch chan Event, msg Event) {
	const maxRetries = 5
	retryDelay := 1 * time.Second
	for i := 0; i < maxRetries; i++ {
		select {
		case ch <- msg:
			return // sent successfully.
		default:
			log.Printf("retry sending message, attempt %d", i)
			time.Sleep(retryDelay)
		}
	}
	log.Println("failed to send message after retries")
}

func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")

	switch origin {
	// TODO: change this
	case "100.73.83.66:8080":
		return true
	default:
		return false
	}
}
