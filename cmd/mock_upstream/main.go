package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

type echoResponse struct {
	Service string              `json:"service"`
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Headers map[string][]string `json:"headers"`
	Message string              `json:"message"`
}

func makeHandler(serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := echoResponse{
			Service: serviceName,
			Method:  r.Method,
			Path:    r.URL.Path,
			Headers: r.Header,
			Message: fmt.Sprintf("Hello from %s downstream service!", serviceName),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func startService(port string, name string, wg *sync.WaitGroup) {
	defer wg.Done()
	mux := http.NewServeMux()
	mux.HandleFunc("/", makeHandler(name))
	log.Printf("[%s] listening on :%s", name, port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Printf("[%s] error: %v", name, err)
	}
}

func main() {
	var wg sync.WaitGroup
	wg.Add(3)
	go startService("9001", "users-service", &wg)
	go startService("9002", "orders-service", &wg)
	go startService("9003", "public-service", &wg)
	wg.Wait()
}
