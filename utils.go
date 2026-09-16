package main

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
)

const R = float64(6_371) //Earth radius in km

// Implement the Haversine function
func calculateDistance(fromLat float64, fromLon float64, toLat float64, toLon float64) float32 {

	fromLatRadians := float64(fromLat * math.Pi / 180)
	fromLonRadians := float64(fromLon * math.Pi / 180)

	toLatRadians := float64(toLat * math.Pi / 180)
	toLonRadians := float64(toLon * math.Pi / 180)

	deltaLat := toLatRadians - fromLatRadians
	deltaLon := toLonRadians - fromLonRadians

	a := math.Pow(math.Sin(deltaLat/2), 2) + math.Cos(fromLatRadians)*math.Cos(toLatRadians)*math.Pow(math.Sin(deltaLon/2), 2)

	c := 2 * math.Asin(math.Sqrt(a))

	return float32(R * c)
}

func findCheapestGasType(stations []GasStation, gasType string) GasStation {

	var cheapest GasStation
	price := float32(math.MaxFloat32)

	for _, gs := range stations {
		isCheaper, p := isCheaperThan(price, gs, gasType)
		if isCheaper {
			cheapest = gs
			price = p
		}
	}

	return cheapest

}

func isCheaperThan(price float32, compare GasStation, gasType string) (bool, float32) {

	potentialNewPrice := float32(math.MaxFloat32)

	for _, pl := range compare.PumpList {
		if strings.ToLower(pl.Type) == gasType {
			if pl.Price < potentialNewPrice {
				potentialNewPrice = pl.Price
			}
		}
	}

	if potentialNewPrice < price {
		return true, potentialNewPrice
	}

	return false, price
}

func fetchClientIP(r *http.Request, hops int, trueIPHeader string) (net.IP, error) {

	// hops == 0: trueIPHeader is expected to be a single-IP header (e.g. a
	// CDN's CF-Connecting-IP), not a chain — no splitting/indexing needed,
	// and no fallback to X-Forwarded-For
	if hops == 0 {
		h := r.Header.Get(trueIPHeader)
		if h == "" {
			return nil, fmt.Errorf("expected %q header to be set when hops is 0", trueIPHeader)
		}
		ip := net.ParseIP(strings.TrimSpace(h))
		if ip == nil {
			return nil, fmt.Errorf("invalid IP in %q header: %q", trueIPHeader, h)
		}
		return ip, nil
	}

	var raw string

	h := r.Header.Get(trueIPHeader)
	if h == "" {
		//fallback on X-Forwarded-For
		h = r.Header.Get("X-Forwarded-For")
	}

	ips := strings.Split(h, ",")

	if hops > 0 {
		if hops > len(ips) {
			return nil, fmt.Errorf("hops (%d) is greater than the number of IPs in the header (%d)", hops, len(ips))
		}
		raw = ips[len(ips)-hops]
	} else {
		raw = ips[len(ips)-1] //take the righest most ip
	}

	parsedIP := net.ParseIP(strings.TrimSpace(raw))
	if parsedIP == nil {
		return nil, fmt.Errorf("invalid IP %q in header", raw)
	}

	return parsedIP, nil
}
