package main

import (
	"encoding/json"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"
)

var fuelTypes = []string{"gasolio", "benzina", "metano", "gpl"}

func gasStationHandler(svc *GasStationService, maxmind *MaxmindService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		municipality := r.URL.Query().Get("municipality")
		ip := r.URL.Query().Get("ip")
		var input Coordinates

		switch {
		case municipality != "":
			input.City = strings.ToLower(municipality)
		case ip != "":
			resolved, err := maxmind.resolveIp(ip)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			input = resolved
		default:
			//fallback on resolving the client ip from the request
			parsedIP, err := fetchClientIP(r, 1, "X-Forwarded-For")
			if err != nil {
				http.Error(w, "Missing municipality/ip parameter and cannot resolve client IP", http.StatusBadRequest)
				return
			}
			resolved, err := maxmind.resolveIp(parsedIP.String())
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			input = resolved
		}

		fuelType := r.URL.Query().Get("type")
		if fuelType == "" {
			http.Error(w, "Missing type parameter", http.StatusBadRequest)
			return
		}
		if !slices.Contains(fuelTypes, strings.ToLower(fuelType)) {
			http.Error(w, "Invalid type parameter", http.StatusBadRequest)
			return
		}

		l := svc.findCheapestGasStationInProximity(input, strings.ToLower(fuelType))
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
	mux.HandleFunc("GET /api/station", gasStationHandler(gasStationSvc, maxMindSvc))

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Println("listening on :8080")
	log.Fatal(server.ListenAndServe())
}
