package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func main() {
	addr := flag.String("addr", "localhost:8443", "Server address")
	token := flag.String("token", "", "JWT Token")
	flag.Parse()

	if *token == "" {
		log.Fatal("Token is required")
	}

	u := fmt.Sprintf("wss://%s/ws?token=%s", *addr, *token)
	log.Printf("Connecting to %s", u)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	// Configure TLS to skip verify for self-signed certs
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	opts := &websocket.DialOptions{
		HTTPClient: httpClient,
	}

	c, _, err := websocket.Dial(ctx, u, opts)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer c.Close(websocket.StatusInternalError, "client closing")

	log.Println("Connected! Waiting for messages...")

	for {
		var v interface{}
		err = wsjson.Read(ctx, c, &v)
		if err != nil {
			log.Printf("Error reading: %v", err)
			return
		}
		log.Printf("RECEIVED: %v", v)
	}
}
