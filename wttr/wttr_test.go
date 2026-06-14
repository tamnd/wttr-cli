package wttr_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tamnd/wttr-cli/wttr"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *wttr.Client {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	cfg := wttr.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return wttr.NewClient(cfg)
}

const sampleWeather = `{
	"current_condition":[{
		"temp_C":"21","temp_F":"70","FeelsLikeC":"18","FeelsLikeF":"64",
		"weatherDesc":[{"value":"Light rain shower"}],
		"windspeedKmph":"9","humidity":"88","visibility":"10",
		"pressure":"1012","cloudcover":"75","uvIndex":"4"
	}],
	"weather":[{
		"date":"2026-06-14","maxtempC":"23","mintempC":"18",
		"hourly":[]
	}],
	"nearest_area":[{
		"areaName":[{"value":"Tokyo"}],
		"country":[{"value":"Japan"}],
		"region":[{"value":"Tokyo"}]
	}]
}`

func TestWeather_userAgent(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if ua == "" {
			t.Error("request carried no User-Agent header")
		}
		if !strings.Contains(ua, "wttr-cli") {
			t.Errorf("User-Agent %q does not contain wttr-cli", ua)
		}
		_, _ = w.Write([]byte(sampleWeather))
	})
	_, err := c.Current(context.Background(), "Tokyo")
	if err != nil {
		t.Fatal(err)
	}
}

func TestWeather_parseTemperatureAndDescription(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleWeather))
	})
	w, err := c.Current(context.Background(), "Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	if w.TempC != "21" {
		t.Errorf("TempC = %q, want 21", w.TempC)
	}
	if w.Description != "Light rain shower" {
		t.Errorf("Description = %q, want Light rain shower", w.Description)
	}
	if w.WindSpeed != "9" {
		t.Errorf("WindSpeed = %q, want 9", w.WindSpeed)
	}
	if w.Country != "Japan" {
		t.Errorf("Country = %q, want Japan", w.Country)
	}
}

func TestWeather_cityInURL(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "Tokyo") {
			t.Errorf("URL path %q does not contain city name", r.URL.Path)
		}
		if r.URL.Query().Get("format") != "j1" {
			t.Errorf("format query param = %q, want j1", r.URL.Query().Get("format"))
		}
		_, _ = w.Write([]byte(sampleWeather))
	})
	_, err := c.Current(context.Background(), "Tokyo")
	if err != nil {
		t.Fatal(err)
	}
}

func TestWeather_retry503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(sampleWeather))
	}))
	defer ts.Close()

	cfg := wttr.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := wttr.NewClient(cfg)

	result, err := c.Current(context.Background(), "London")
	if err != nil {
		t.Fatal(err)
	}
	if result.TempC != "21" {
		t.Errorf("TempC = %q after retry", result.TempC)
	}
	if hits != 3 {
		t.Errorf("server hits = %d, want 3", hits)
	}
}

func TestWeather_unicodeCityName(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// URL should be percent-encoded; path should be non-empty
		if r.URL.Path == "/" || r.URL.Path == "" {
			t.Errorf("URL path %q looks empty for unicode city", r.URL.Path)
		}
		_, _ = w.Write([]byte(sampleWeather))
	})
	// Tokyo in Japanese
	_, err := c.Current(context.Background(), "東京")
	if err != nil {
		t.Fatal(err)
	}
}
