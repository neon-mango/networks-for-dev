package main

import (
	"log"
	"net/http"

	"notes/internal/httpapi"
	"notes/internal/storage"
)

func main() {
	store := storage.New()
	handler := httpapi.NewHandler(store)

	addr := ":8080"
	log.Printf("http/1.1 listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler.Routes()))
}
