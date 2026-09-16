package main

import (
	"fmt"
	"log"
	"net/netip"

	"github.com/oschwald/maxminddb-golang/v2"
)

type MaxmindService struct {
	db *maxminddb.Reader
}

type maxmind_response struct {
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

type Coordinates struct {
	City string  `json:"city"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

func newMaxMindService(path string) *MaxmindService {

	db, err := maxminddb.Open(path)
	if err != nil {
		log.Fatalf("Cannot open the maxmind db file: %s", err)
	}

	return &MaxmindService{
		db: db,
	}
}

func (s MaxmindService) Close() error {
	return s.db.Close()
}

func (s MaxmindService) resolveIp(ip string) (Coordinates, error) {

	resolvedIp, err := netip.ParseAddr(ip)
	if err != nil {
		return Coordinates{}, fmt.Errorf("%q is not a valid ip address: %w", ip, err)
	}

	var record maxmind_response

	if err := s.db.Lookup(resolvedIp).Decode(&record); err != nil {
		return Coordinates{}, fmt.Errorf("error resolving the ip address: %w", err)
	}

	return Coordinates{
		City: record.City.Names["en"],
		Lat:  record.Location.Latitude,
		Lon:  record.Location.Longitude,
	}, nil
}
