package main

import (
	"math"
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
		if pl.Type == gasType {
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
