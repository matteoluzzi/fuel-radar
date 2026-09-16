package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

const stationFieldCount = 10
const maxDistanceInKm = 5

// gestori.prezzibenzina.it is a stray attribution suffix that some source
// records leak into a free-text field (e.g. "STOIL SIMPLE | gestori.prezzibenzina.it"),
// splitting that field in two across the pipe delimiter. normalizeStationRow
// merges those fragments back together so column positions line up again.
const stationRowNoiseMarker = "gestori.prezzibenzina.it"

type GasStationList struct {
	List []GasStation
}

type GasStation struct {
	Id           string
	Brand        string
	Flag         string
	Type         string
	Name         string
	Address      string
	Municipality string
	Province     string
	Latitude     float64
	Longitude    float64
	PumpList     []GasStationPump
}

type GasStationPump struct {
	Type          string
	Price         float32
	IsSelfService bool
	LastUpdated   string
}

type GasStationService struct {
	StationList GasStationList
}

func newGasStationService() *GasStationService {
	priceListRows, err := readCsvFile("gas_station_data/prezzo_alle_8.csv")
	if err != nil {
		log.Fatal(err)
	}

	stationRows, err := readCsvFile("gas_station_data/anagrafica_impianti_attivi.csv")
	if err != nil {
		log.Fatal(err)
	}

	priceListById := parsePriceList(priceListRows)
	stationList := parseGasStations(stationRows, priceListById)

	return &GasStationService{
		StationList: GasStationList{List: stationList},
	}
}

// readCsvFile reads a pipe-delimited export and returns its data rows, split
// on '|', skipping the "Estrazione del ..." line and the column header line
// that precede the data. These exports don't use real CSV quoting (stray "
// characters inside free-text fields are just messy data, not escaping), so
// fields are split per line rather than through encoding/csv's quote-aware
// parser, which mis-parses those stray quotes.
func readCsvFile(path string) ([][]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening %s: %w", path, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("failed to close %s: %s", path, err)
		}
	}()

	scanner := bufio.NewScanner(file)

	if !scanner.Scan() { // "Estrazione del ..." line
		return nil, fmt.Errorf("error reading %s: %w", path, scanner.Err())
	}
	if !scanner.Scan() { // header line
		return nil, fmt.Errorf("error reading %s: %w", path, scanner.Err())
	}

	var rows [][]string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		rows = append(rows, strings.Split(line, "|"))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading %s: %w", path, err)
	}
	return rows, nil
}

// parsePriceList groups pump prices by station id.
// Columns: idImpianto|descCarburante|prezzo|isSelf|dtComu
func parsePriceList(rows [][]string) map[int][]GasStationPump {
	priceListById := make(map[int][]GasStationPump)

	for _, row := range rows {
		id, err := strconv.Atoi(row[0])
		if err != nil {
			log.Fatal("Error parsing station id")
		}
		price, err := strconv.ParseFloat(row[2], 32)
		if err != nil {
			log.Fatal("Error parsing price")
		}
		isSelfService, err := strconv.ParseBool(row[3])
		if err != nil {
			log.Fatal("Error parsing is self service flag")
		}

		priceListById[id] = append(priceListById[id], GasStationPump{
			Type:          row[1],
			Price:         float32(price),
			IsSelfService: isSelfService,
			LastUpdated:   row[4],
		})
	}

	return priceListById
}

// normalizeStationRow repairs rows split into more than stationFieldCount
// fields because of embedded stationRowNoiseMarker fragments, merging each
// noise fragment back into the original field before it in a single pass
// over the raw fields. A single pass matters here: a merged field itself
// contains the marker text, so re-scanning results (rather than the original
// fields) would wrongly merge it again into the field before it.
func normalizeStationRow(row []string) []string {
	result := make([]string, 0, len(row))
	for _, field := range row {
		if len(result) > 0 && strings.Contains(field, stationRowNoiseMarker) {
			result[len(result)-1] += "|" + field
			continue
		}
		result = append(result, field)
	}
	return result
}

// parseGasStations builds the station list, attaching each station's pumps
// from priceListById by matching station id.
// Columns: idImpianto|Gestore|Bandiera|Tipo Impianto|Nome Impianto|Indirizzo|Comune|Provincia|Latitudine|Longitudine
func parseGasStations(rows [][]string, priceListById map[int][]GasStationPump) []GasStation {
	stationList := make([]GasStation, 0, len(rows))

	for _, row := range rows {
		row = normalizeStationRow(row)
		if len(row) != stationFieldCount {
			log.Printf("skipping station row with unexpected field count %d: %v", len(row), row)
			continue
		}

		id, err := strconv.Atoi(row[0])
		if err != nil {
			log.Printf("skipping station row with unparseable id %q", row[0])
			continue
		}
		latitude, err := strconv.ParseFloat(strings.TrimSpace(row[8]), 64)
		if err != nil {
			log.Printf("skipping station %d with unparseable latitude %q", id, row[8])
			continue
		}
		longitude, err := strconv.ParseFloat(strings.TrimSpace(row[9]), 64)
		if err != nil {
			log.Printf("skipping station %d with unparseable longitude %q", id, row[9])
			continue
		}

		stationList = append(stationList, GasStation{
			Id:           row[0],
			Brand:        row[1],
			Flag:         row[2],
			Type:         row[3],
			Name:         row[4],
			Address:      row[5],
			Municipality: row[6],
			Province:     row[7],
			Latitude:     latitude,
			Longitude:    longitude,
			PumpList:     priceListById[id],
		})
	}

	return stationList
}

func (svc GasStationService) findAllGasStationinMunicipality(municipality string) []GasStation {
	result := make([]GasStation, 0)

	for _, gs := range svc.StationList.List {
		if strings.ToLower(gs.Municipality) == municipality {
			result = append(result, gs)
		}
	}
	return result
}

func (svc GasStationService) findCheapestGasStationInProximity(input Coordinates, gasType string) GasStation {

	var gasStationsInProximity []GasStation

	if input.Lat == 0 || input.Lon == 0 {
		//Use the city
		gasStationsInProximity = svc.findAllGasStationinMunicipality(input.City)
	} else {
		for _, gs := range svc.StationList.List {
			if maxDistanceInKm > calculateDistance(input.Lat, input.Lon, gs.Latitude, gs.Longitude) {
				gasStationsInProximity = append(gasStationsInProximity, gs)
			}
		}
	}

	return findCheapestGasType(gasStationsInProximity, gasType)
}
