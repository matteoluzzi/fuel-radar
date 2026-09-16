package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
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

func gasStationHandler(svc *GasStationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		municipality := r.URL.Query().Get("municipality")
		if municipality == "" {
			http.Error(w, "Missing municipality parameter", http.StatusBadRequest)
			return
		}

		l := svc.findAllGasStationinMunicipality(strings.ToLower(municipality))
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(l); err != nil {
			log.Printf("failed to write response: %s", err)
		}

	}
}

func main() {
	maxMindSvc := newMaxMindService("GeoLite2-City.mmdb")
	gasStationSvc := newGasStationService()
	defer func() {
		if err := maxMindSvc.Close(); err != nil {
			log.Printf("failed to close maxmind db: %s", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/location", locationHandler(maxMindSvc))
	mux.HandleFunc("GET /api/station", gasStationHandler(&gasStationSvc))

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Println("listening on :8080")
	log.Fatal(server.ListenAndServe())
}
