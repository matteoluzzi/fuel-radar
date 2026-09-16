package main

import (
	"math"
	"testing"
)

const distanceToleranceKm = 0.01

type coordinates struct {
	lat float64
	lon float64
}

func TestHaversineDistance(t *testing.T) {
	tests := []struct {
		name string
		from coordinates
		to   coordinates
		want float32
	}{
		{
			name: "Test case 1",
			from: coordinates{
				lat: 37.312390927445705,
				lon: 13.58591309570079,
			},
			to: coordinates{
				lat: 44.93516582629164,
				lon: 8.883669263274442,
			},
			want: 934.23,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateDistance(tt.from.lat, tt.from.lon, tt.to.lat, tt.to.lon)
			if diff := math.Abs(float64(result - tt.want)); diff > distanceToleranceKm {
				t.Errorf("got %f, want %f (diff %f exceeds tolerance %f)", result, tt.want, diff, distanceToleranceKm)
			}
		})
	}
}
