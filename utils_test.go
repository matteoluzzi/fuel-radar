package main

import (
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func mustParseInRome(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, datasetTimezone)
	if err != nil {
		t.Fatalf("failed to parse time %q: %s", value, err)
	}
	return parsed
}

func TestLastDatasetUpdateTime(t *testing.T) {
	tests := []struct {
		name string
		now  string // parsed in Europe/Rome
		want string // parsed in Europe/Rome
	}{
		{
			name: "just before 8am rolls back to the previous day",
			now:  "2026-03-10 07:59:59",
			want: "2026-03-09 08:00:00",
		},
		{
			name: "exactly 8am counts as already updated",
			now:  "2026-03-10 08:00:00",
			want: "2026-03-10 08:00:00",
		},
		{
			name: "later in the day keeps the same day's cutoff",
			now:  "2026-03-10 23:59:59",
			want: "2026-03-10 08:00:00",
		},
		{
			name: "just after midnight rolls back to the previous day",
			now:  "2026-03-10 00:00:01",
			want: "2026-03-09 08:00:00",
		},
		{
			name: "CEST/CET transition day still cuts off at local 8am",
			now:  "2026-10-25 09:00:00",
			want: "2026-10-25 08:00:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := mustParseInRome(t, tt.now)
			want := mustParseInRome(t, tt.want)

			got := lastDatasetUpdateTime(now)
			if !got.Equal(want) {
				t.Errorf("lastDatasetUpdateTime(%s) = %s, want %s", now, got, want)
			}
		})
	}
}

func TestLastDatasetUpdateTime_NormalizesInputTimezone(t *testing.T) {
	// 07:30 in Rome (winter, UTC+1) is 06:30 UTC; the cutoff must be computed
	// on the Rome wall-clock day, not on whatever zone `now` arrives in.
	nowUTC := time.Date(2026, time.January, 10, 6, 30, 0, 0, time.UTC)
	want := mustParseInRome(t, "2026-01-09 08:00:00")

	got := lastDatasetUpdateTime(nowUTC)
	if !got.Equal(want) {
		t.Errorf("lastDatasetUpdateTime(%s) = %s, want %s", nowUTC, got, want)
	}
}

func TestDownloadDataset(t *testing.T) {
	t.Run("writes the response body to disk", func(t *testing.T) {
		const body = "idImpianto|descCarburante|prezzo|isSelf|dtComu\n1|benzina|1.899|true|2026-01-01"

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		path := filepath.Join(t.TempDir(), "dataset.csv")
		if err := downloadDataset(server.URL, path); err != nil {
			t.Fatalf("downloadDataset returned error: %s", err)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read downloaded file: %s", err)
		}
		if string(got) != body {
			t.Errorf("got file content %q, want %q", got, body)
		}
	})

	t.Run("returns an error when the request fails", func(t *testing.T) {
		if err := downloadDataset("http://127.0.0.1:0", filepath.Join(t.TempDir(), "dataset.csv")); err == nil {
			t.Error("expected an error, got nil")
		}
	})

	t.Run("returns an error when the destination cannot be created", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("data"))
		}))
		defer server.Close()

		// the parent directory does not exist, so os.Create must fail
		badPath := filepath.Join(t.TempDir(), "missing-dir", "dataset.csv")
		if err := downloadDataset(server.URL, badPath); err == nil {
			t.Error("expected an error, got nil")
		}
	})
}

func TestDownloadDatasetFileIfStale(t *testing.T) {
	const initialBody = "initial"
	const freshBody = "refreshed"

	t.Run("downloads when the file does not exist", func(t *testing.T) {
		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			_, _ = w.Write([]byte(freshBody))
		}))
		defer server.Close()

		path := filepath.Join(t.TempDir(), "dataset.csv")
		if err := downloadDatasetFileIfStale(server.URL, path); err != nil {
			t.Fatalf("downloadDatasetFileIfStale returned error: %s", err)
		}
		if requests != 1 {
			t.Errorf("expected 1 request, got %d", requests)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read downloaded file: %s", err)
		}
		if string(got) != freshBody {
			t.Errorf("got file content %q, want %q", got, freshBody)
		}
	})

	t.Run("re-downloads a stale file", func(t *testing.T) {
		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			_, _ = w.Write([]byte(freshBody))
		}))
		defer server.Close()

		path := filepath.Join(t.TempDir(), "dataset.csv")
		if err := os.WriteFile(path, []byte(initialBody), 0o600); err != nil {
			t.Fatalf("failed to seed file: %s", err)
		}

		staleTime := lastDatasetUpdateTime(time.Now()).Add(-time.Minute)
		if err := os.Chtimes(path, staleTime, staleTime); err != nil {
			t.Fatalf("failed to set mod time: %s", err)
		}

		if err := downloadDatasetFileIfStale(server.URL, path); err != nil {
			t.Fatalf("downloadDatasetFileIfStale returned error: %s", err)
		}
		if requests != 1 {
			t.Errorf("expected 1 request, got %d", requests)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read downloaded file: %s", err)
		}
		if string(got) != freshBody {
			t.Errorf("got file content %q, want %q", got, freshBody)
		}
	})

	t.Run("keeps a fresh file untouched", func(t *testing.T) {
		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			_, _ = w.Write([]byte(freshBody))
		}))
		defer server.Close()

		path := filepath.Join(t.TempDir(), "dataset.csv")
		if err := os.WriteFile(path, []byte(initialBody), 0o600); err != nil {
			t.Fatalf("failed to seed file: %s", err)
		}

		freshTime := lastDatasetUpdateTime(time.Now()).Add(time.Minute)
		if err := os.Chtimes(path, freshTime, freshTime); err != nil {
			t.Fatalf("failed to set mod time: %s", err)
		}

		if err := downloadDatasetFileIfStale(server.URL, path); err != nil {
			t.Fatalf("downloadDatasetFileIfStale returned error: %s", err)
		}
		if requests != 0 {
			t.Errorf("expected no requests, got %d", requests)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %s", err)
		}
		if string(got) != initialBody {
			t.Errorf("file was modified, got %q, want %q", got, initialBody)
		}
	})

	t.Run("propagates non-not-exist stat errors", func(t *testing.T) {
		// path treats a regular file as a directory component, so os.Stat
		// fails with something other than "not exist" (e.g. ENOTDIR).
		parent := filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(parent, []byte("x"), 0o600); err != nil {
			t.Fatalf("failed to seed file: %s", err)
		}
		path := filepath.Join(parent, "dataset.csv")

		err := downloadDatasetFileIfStale("http://example.invalid", path)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if os.IsNotExist(err) {
			t.Errorf("expected a non-not-exist error, got %s", err)
		}
	})
}
