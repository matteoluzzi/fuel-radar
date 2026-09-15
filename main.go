package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"
)

func locationHandler(svc *MaxmindService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.URL.Query().Get("ip")
		if ip == "" {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			ip = host
		}

		resp, err := svc.resolveIp(ip)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("failed to write response: %s", err)
		}
	}
}

func main() {
	svc := newService("GeoLite2-City.mmdb")
	defer func() {
		if err := svc.Close(); err != nil {
			log.Printf("failed to close maxmind db: %s", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/location", locationHandler(svc))

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Println("listening on :8080")
	log.Fatal(server.ListenAndServe())
}
