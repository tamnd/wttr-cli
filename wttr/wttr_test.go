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

const sampleResponse = `{
	"current_condition":[{
		"temp_C":"21","temp_F":"70","FeelsLikeC":"18","FeelsLikeF":"64",
		"weatherDesc":[{"value":"Light rain shower"}],
		"windspeedKmph":"9","winddir16Point":"SW","humidity":"88","visibility":"10",
		"pressure":"1012","cloudcover":"75","uvIndex":"4"
	}],
	"weather":[
		{
			"date":"2026-06-14","maxtempC":"23","mintempC":"18","maxtempF":"73","mintempF":"64",
			"astronomy":[{"sunrise":"05:47 AM","sunset":"09:56 PM"}],
			"hourly":[]
		},
		{
			"date":"2026-06-15","maxtempC":"24","mintempC":"17","maxtempF":"75","mintempF":"63",
			"astronomy":[{"sunrise":"05:47 AM","sunset":"09:57 PM"}],
			"hourly":[]
		},
		{
			"date":"2026-06-16","maxtempC":"22","mintempC":"16","maxtempF":"72","mintempF":"61",
			"astronomy":[{"sunrise":"05:47 AM","sunset":"09:57 PM"}],
			"hourly":[]
		}
	],
	"nearest_area":[{
		"areaName":[{"value":"Tokyo"}],
		"country":[{"value":"Japan"}],
		"region":[{"value":"Tokyo"}]
	}]
}`

func TestCurrent_userAgent(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if ua == "" {
			t.Error("request carried no User-Agent header")
		}
		if !strings.Contains(ua, "wttr-cli") {
			t.Errorf("User-Agent %q does not contain wttr-cli", ua)
		}
		_, _ = w.Write([]byte(sampleResponse))
	})
	_, err := c.Current(context.Background(), "Tokyo")
	if err != nil {
		t.Fatal(err)
	}
}

func TestCurrent_parseFields(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleResponse))
	})
	cur, err := c.Current(context.Background(), "Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	if cur.TempC != "21" {
		t.Errorf("TempC = %q, want 21", cur.TempC)
	}
	if cur.TempF != "70" {
		t.Errorf("TempF = %q, want 70", cur.TempF)
	}
	if cur.FeelsLikeC != "18" {
		t.Errorf("FeelsLikeC = %q, want 18", cur.FeelsLikeC)
	}
	if cur.Description != "Light rain shower" {
		t.Errorf("Description = %q, want Light rain shower", cur.Description)
	}
	if cur.WindKmph != "9" {
		t.Errorf("WindKmph = %q, want 9", cur.WindKmph)
	}
	if cur.WindDir != "SW" {
		t.Errorf("WindDir = %q, want SW", cur.WindDir)
	}
	if cur.Humidity != "88" {
		t.Errorf("Humidity = %q, want 88", cur.Humidity)
	}
	if cur.Visibility != "10" {
		t.Errorf("Visibility = %q, want 10", cur.Visibility)
	}
	if cur.UV != "4" {
		t.Errorf("UV = %q, want 4", cur.UV)
	}
	if cur.Location != "Tokyo, Japan" {
		t.Errorf("Location = %q, want Tokyo, Japan", cur.Location)
	}
}

func TestCurrent_locationInURL(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "Tokyo") {
			t.Errorf("URL path %q does not contain location", r.URL.Path)
		}
		if r.URL.Query().Get("format") != "j1" {
			t.Errorf("format query param = %q, want j1", r.URL.Query().Get("format"))
		}
		_, _ = w.Write([]byte(sampleResponse))
	})
	_, err := c.Current(context.Background(), "Tokyo")
	if err != nil {
		t.Fatal(err)
	}
}

func TestForecast_threeDays(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleResponse))
	})
	days, err := c.Forecast(context.Background(), "Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 3 {
		t.Fatalf("got %d forecast days, want 3", len(days))
	}
	d := days[0]
	if d.Date != "2026-06-14" {
		t.Errorf("days[0].Date = %q, want 2026-06-14", d.Date)
	}
	if d.MaxC != "23" {
		t.Errorf("days[0].MaxC = %q, want 23", d.MaxC)
	}
	if d.MinC != "18" {
		t.Errorf("days[0].MinC = %q, want 18", d.MinC)
	}
	if d.MaxF != "73" {
		t.Errorf("days[0].MaxF = %q, want 73", d.MaxF)
	}
	if d.MinF != "64" {
		t.Errorf("days[0].MinF = %q, want 64", d.MinF)
	}
	if d.Sunrise != "05:47 AM" {
		t.Errorf("days[0].Sunrise = %q, want 05:47 AM", d.Sunrise)
	}
	if d.Sunset != "09:56 PM" {
		t.Errorf("days[0].Sunset = %q, want 09:56 PM", d.Sunset)
	}
}

func TestCurrent_retry503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(sampleResponse))
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

func TestCurrent_unicodeCityName(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "" {
			t.Errorf("URL path %q looks empty for unicode city", r.URL.Path)
		}
		_, _ = w.Write([]byte(sampleResponse))
	})
	// Tokyo in Japanese
	_, err := c.Current(context.Background(), "東京")
	if err != nil {
		t.Fatal(err)
	}
}

func TestForecast_retry503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(sampleResponse))
	}))
	defer ts.Close()

	cfg := wttr.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 3
	c := wttr.NewClient(cfg)

	days, err := c.Forecast(context.Background(), "Paris")
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 3 {
		t.Errorf("got %d forecast days after retry", len(days))
	}
}
