package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Response struct {
	Message   string            `json:"message"`
	Path      string            `json:"path"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	Timestamp string            `json:"timestamp"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	// Collect headers
	headers := make(map[string]string)
	for name, values := range r.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	response := Response{
		Message:   "Hello from test backend service!",
		Path:      r.URL.Path,
		Method:    r.Method,
		Headers:   headers,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	log.Printf("%s %s - Forwarded from: %s", r.Method, r.URL.Path, r.Header.Get("X-Forwarded-For"))
}

func main() {
	http.HandleFunc("/", handler)

	addr := ":8081"
	log.Printf("Test backend service running on %s", addr)
	log.Printf("Try: curl http://localhost:8081/anything")

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
