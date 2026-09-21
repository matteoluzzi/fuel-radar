package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeCsvFixture(t *testing.T, lines ...string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fixture.csv")
	content := "Estrazione del 01/01/2026\nheader|line\n"
	for _, l := range lines {
		content += l + "\n"
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write fixture: %s", err)
	}
	return path
}

func TestReadCsvFile(t *testing.T) {
	t.Run("skips the extraction and header lines", func(t *testing.T) {
		path := writeCsvFixture(t, "1|benzina|1.899|true|2026-01-01", "2|gasolio|1.799|false|2026-01-01")

		rows, err := readCsvFile(path)
		if err != nil {
			t.Fatalf("readCsvFile returned error: %s", err)
		}

		want := [][]string{
			{"1", "benzina", "1.899", "true", "2026-01-01"},
			{"2", "gasolio", "1.799", "false", "2026-01-01"},
		}
		if !reflect.DeepEqual(rows, want) {
			t.Errorf("got %v, want %v", rows, want)
		}
	})

	t.Run("skips blank lines", func(t *testing.T) {
		path := writeCsvFixture(t, "1|benzina|1.899|true|2026-01-01", "", "2|gasolio|1.799|false|2026-01-01")

		rows, err := readCsvFile(path)
		if err != nil {
			t.Fatalf("readCsvFile returned error: %s", err)
		}
		if len(rows) != 2 {
			t.Errorf("got %d rows, want 2", len(rows))
		}
	})

	t.Run("returns an error when the file does not exist", func(t *testing.T) {
		_, err := readCsvFile(filepath.Join(t.TempDir(), "missing.csv"))
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})

	t.Run("returns an error when the header line is missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "fixture.csv")
		if err := os.WriteFile(path, []byte("Estrazione del 01/01/2026\n"), 0o600); err != nil {
			t.Fatalf("failed to write fixture: %s", err)
		}

		_, err := readCsvFile(path)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})
}

func TestNormalizeStationRow(t *testing.T) {
	tests := []struct {
		name string
		row  []string
		want []string
	}{
		{
			name: "row without noise markers is unchanged",
			row:  []string{"1", "Gestore", "Bandiera", "Tipo", "Nome", "Indirizzo", "Comune", "Provincia", "44.0", "8.0"},
			want: []string{"1", "Gestore", "Bandiera", "Tipo", "Nome", "Indirizzo", "Comune", "Provincia", "44.0", "8.0"},
		},
		{
			name: "a noise fragment is merged back into the preceding field",
			row:  []string{"1", "Gestore", "STOIL SIMPLE ", " gestori.prezzibenzina.it", "Tipo", "Nome", "Indirizzo", "Comune", "Provincia", "44.0", "8.0"},
			want: []string{"1", "Gestore", "STOIL SIMPLE | gestori.prezzibenzina.it", "Tipo", "Nome", "Indirizzo", "Comune", "Provincia", "44.0", "8.0"},
		},
		{
			name: "a leading noise fragment has nothing to merge into",
			row:  []string{" gestori.prezzibenzina.it", "Gestore"},
			want: []string{" gestori.prezzibenzina.it", "Gestore"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeStationRow(tt.row)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePriceList(t *testing.T) {
	rows := [][]string{
		{"1", "benzina", "1.899", "true", "2026-01-01 08:00"},
		{"1", "gasolio", "1.799", "false", "2026-01-01 08:00"},
		{"2", "benzina", "1.999", "true", "2026-01-01 08:00"},
	}

	got := parsePriceList(rows)

	if len(got[1]) != 2 {
		t.Fatalf("expected 2 pumps for station 1, got %d", len(got[1]))
	}
	if len(got[2]) != 1 {
		t.Fatalf("expected 1 pump for station 2, got %d", len(got[2]))
	}

	want1 := []GasStationPump{
		{Type: "benzina", Price: 1.899, IsSelfService: true, LastUpdated: "2026-01-01 08:00"},
		{Type: "gasolio", Price: 1.799, IsSelfService: false, LastUpdated: "2026-01-01 08:00"},
	}
	if !reflect.DeepEqual(got[1], want1) {
		t.Errorf("got %v, want %v", got[1], want1)
	}
}

func TestParseGasStations(t *testing.T) {
	priceListById := map[int][]GasStationPump{
		1: {{Type: "benzina", Price: 1.899}},
	}

	t.Run("parses a valid row and attaches its pumps", func(t *testing.T) {
		rows := [][]string{
			{"1", "Gestore", "Bandiera", "Tipo", "Nome", "Indirizzo", "Comune", "Provincia", "44.0", "8.0"},
		}

		stations := parseGasStations(rows, priceListById)
		if len(stations) != 1 {
			t.Fatalf("got %d stations, want 1", len(stations))
		}

		want := GasStation{
			Id: "1", Brand: "Gestore", Flag: "Bandiera", Type: "Tipo", Name: "Nome",
			Address: "Indirizzo", Municipality: "Comune", Province: "Provincia",
			Latitude: 44.0, Longitude: 8.0, PumpList: priceListById[1],
		}
		if !reflect.DeepEqual(stations[0], want) {
			t.Errorf("got %+v, want %+v", stations[0], want)
		}
	})

	t.Run("reassembles a row split by the noise marker before validating field count", func(t *testing.T) {
		rows := [][]string{
			{"1", "Gestore", "STOIL SIMPLE ", " gestori.prezzibenzina.it", "Tipo", "Nome", "Indirizzo", "Comune", "Provincia", "44.0", "8.0"},
		}

		stations := parseGasStations(rows, priceListById)
		if len(stations) != 1 {
			t.Fatalf("got %d stations, want 1", len(stations))
		}
		if stations[0].Flag != "STOIL SIMPLE | gestori.prezzibenzina.it" {
			t.Errorf("got Flag %q, want merged noise marker fragment", stations[0].Flag)
		}
	})

	t.Run("skips a row with the wrong field count", func(t *testing.T) {
		rows := [][]string{
			{"1", "Gestore", "Bandiera"},
		}

		stations := parseGasStations(rows, priceListById)
		if len(stations) != 0 {
			t.Errorf("got %d stations, want 0", len(stations))
		}
	})

	t.Run("skips a row with an unparseable id", func(t *testing.T) {
		rows := [][]string{
			{"not-an-id", "Gestore", "Bandiera", "Tipo", "Nome", "Indirizzo", "Comune", "Provincia", "44.0", "8.0"},
		}

		stations := parseGasStations(rows, priceListById)
		if len(stations) != 0 {
			t.Errorf("got %d stations, want 0", len(stations))
		}
	})

	t.Run("skips a row with an unparseable latitude", func(t *testing.T) {
		rows := [][]string{
			{"1", "Gestore", "Bandiera", "Tipo", "Nome", "Indirizzo", "Comune", "Provincia", "not-a-lat", "8.0"},
		}

		stations := parseGasStations(rows, priceListById)
		if len(stations) != 0 {
			t.Errorf("got %d stations, want 0", len(stations))
		}
	})

	t.Run("skips a row with an unparseable longitude", func(t *testing.T) {
		rows := [][]string{
			{"1", "Gestore", "Bandiera", "Tipo", "Nome", "Indirizzo", "Comune", "Provincia", "44.0", "not-a-lon"},
		}

		stations := parseGasStations(rows, priceListById)
		if len(stations) != 0 {
			t.Errorf("got %d stations, want 0", len(stations))
		}
	})
}

func TestIsCheaperThan(t *testing.T) {
	tests := []struct {
		name        string
		price       float32
		compare     GasStation
		gasType     string
		wantCheaper bool
		wantPrice   float32
	}{
		{
			name:  "a lower price for the matching gas type is cheaper",
			price: 1.899,
			compare: GasStation{PumpList: []GasStationPump{
				{Type: "Benzina", Price: 1.799},
			}},
			gasType:     "benzina",
			wantCheaper: true,
			wantPrice:   1.799,
		},
		{
			name:  "a higher price is not cheaper",
			price: 1.699,
			compare: GasStation{PumpList: []GasStationPump{
				{Type: "Benzina", Price: 1.799},
			}},
			gasType:     "benzina",
			wantCheaper: false,
			wantPrice:   1.699,
		},
		{
			name:  "a station without the requested gas type is not cheaper",
			price: 1.899,
			compare: GasStation{PumpList: []GasStationPump{
				{Type: "Gasolio", Price: 1.5},
			}},
			gasType:     "benzina",
			wantCheaper: false,
			wantPrice:   1.899,
		},
		{
			name:  "the cheapest matching pump is picked among several",
			price: 1.899,
			compare: GasStation{PumpList: []GasStationPump{
				{Type: "Benzina", Price: 1.799},
				{Type: "Benzina", Price: 1.699},
			}},
			gasType:     "benzina",
			wantCheaper: true,
			wantPrice:   1.699,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cheaper, price := isCheaperThan(tt.price, tt.compare, tt.gasType)
			if cheaper != tt.wantCheaper {
				t.Errorf("got cheaper=%v, want %v", cheaper, tt.wantCheaper)
			}
			if price != tt.wantPrice {
				t.Errorf("got price=%v, want %v", price, tt.wantPrice)
			}
		})
	}
}

func TestFindCheapestGasType(t *testing.T) {
	stations := []GasStation{
		{Id: "1", PumpList: []GasStationPump{{Type: "benzina", Price: 1.899}}},
		{Id: "2", PumpList: []GasStationPump{{Type: "benzina", Price: 1.699}}},
		{Id: "3", PumpList: []GasStationPump{{Type: "gasolio", Price: 1.5}}},
	}

	got := findCheapestGasType(stations, "benzina")
	if got.Id != "2" {
		t.Errorf("got station %q, want %q", got.Id, "2")
	}
}

func TestFindAllGasStationinMunicipality(t *testing.T) {
	svc := GasStationService{StationList: GasStationList{List: []GasStation{
		{Id: "1", Municipality: "Milano"},
		{Id: "2", Municipality: "Roma"},
		{Id: "3", Municipality: "Milano"},
	}}}

	got := svc.findAllGasStationinMunicipality("milano")
	if len(got) != 2 {
		t.Fatalf("got %d stations, want 2", len(got))
	}
	for _, gs := range got {
		if gs.Municipality != "Milano" {
			t.Errorf("got station in %q, want Milano", gs.Municipality)
		}
	}
}

func TestFindCheapestGasStationInProximity(t *testing.T) {
	t.Run("falls back to the municipality when no coordinates are given", func(t *testing.T) {
		svc := GasStationService{StationList: GasStationList{List: []GasStation{
			{Id: "1", Municipality: "Milano", PumpList: []GasStationPump{{Type: "benzina", Price: 1.899}}},
			{Id: "2", Municipality: "Milano", PumpList: []GasStationPump{{Type: "benzina", Price: 1.699}}},
			{Id: "3", Municipality: "Roma", PumpList: []GasStationPump{{Type: "benzina", Price: 1.499}}},
		}}}

		got := svc.findCheapestGasStationInProximity(Coordinates{City: "milano"}, "benzina")
		if got.Id != "2" {
			t.Errorf("got station %q, want %q", got.Id, "2")
		}
	})

	t.Run("uses coordinates to restrict to stations within range", func(t *testing.T) {
		// Milan city center
		const milanLat, milanLon = 45.4642, 9.1900

		svc := GasStationService{StationList: GasStationList{List: []GasStation{
			{Id: "near-cheap", Latitude: milanLat, Longitude: milanLon, PumpList: []GasStationPump{{Type: "benzina", Price: 1.699}}},
			{Id: "near-expensive", Latitude: milanLat + 0.001, Longitude: milanLon, PumpList: []GasStationPump{{Type: "benzina", Price: 1.899}}},
			// Rome, well outside maxDistanceInKm from Milan
			{Id: "far-cheapest", Latitude: 41.9028, Longitude: 12.4964, PumpList: []GasStationPump{{Type: "benzina", Price: 0.999}}},
		}}}

		got := svc.findCheapestGasStationInProximity(Coordinates{Lat: milanLat, Lon: milanLon}, "benzina")
		if got.Id != "near-cheap" {
			t.Errorf("got station %q, want %q", got.Id, "near-cheap")
		}
	})
}
