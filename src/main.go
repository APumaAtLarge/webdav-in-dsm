package main

import (
	"log"
	"net/http"
	"os"

	"github.com/apumaatlarge/webdav-in-dsm/src/contorller"
)

func main() {
	addr := getenv("ADDR", ":8080")
	log.Printf("link api listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, contorller.NewMux()))
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
