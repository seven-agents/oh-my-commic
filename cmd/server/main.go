package main

import (
	"log"
	"net/http"

	"github.com/seven-agents/oh-my-commic/internal/httpx"
)

func main() {
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", httpx.NewRouter()))
}
