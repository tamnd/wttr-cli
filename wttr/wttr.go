// Package wttr is the library behind the wttr command line:
// the HTTP client, request shaping, and the typed data models for the
// wttr.in API (weather for any city, no key required).
//
// The Client is the spine every command shares. It sets a real User-Agent,
// paces requests so a busy session stays polite, and retries the transient
// failures (429 and 5xx) that any public API throws under load.
package wttr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Host is the site this client talks to.
const Host = "wttr.in"

// Config holds all tuneable parameters for a Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns the production configuration for wttr.in.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://wttr.in",
		UserAgent: "wttr-cli/0.1.0 (github.com/tamnd/wttr-cli)",
		Rate:      200 * time.Millisecond,
		Timeout:   30 * time.Second,
		Retries:   3,
	}
}

// Weather holds the current conditions and today's forecast for one city.
type Weather struct {
	City        string `json:"city"`
	TempC       string `json:"temp_c"`
	TempF       string `json:"temp_f"`
	FeelsLikeC  string `json:"feels_like_c"`
	Description string `json:"description"`
	WindSpeed   string `json:"wind_speed_kmph"`
	Humidity    string `json:"humidity"`
	Visibility  string `json:"visibility"`
	Pressure    string `json:"pressure"`
	UV          string `json:"uv_index"`
	MaxTempC    string `json:"max_temp_c"`
	MinTempC    string `json:"min_temp_c"`
	Date        string `json:"date"`
	Country     string `json:"country"`
	Region      string `json:"region"`
}

// Client talks to wttr.in over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured from cfg.
func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}
}

// raw JSON shapes for parsing the j1 format.
type wttrJ1 struct {
	CurrentCondition []struct {
		TempC       string `json:"temp_C"`
		TempF       string `json:"temp_F"`
		FeelsLikeC  string `json:"FeelsLikeC"`
		WeatherDesc []struct {
			Value string `json:"value"`
		} `json:"weatherDesc"`
		WindspeedKmph string `json:"windspeedKmph"`
		Humidity      string `json:"humidity"`
		Visibility    string `json:"visibility"`
		Pressure      string `json:"pressure"`
		UVIndex       string `json:"uvIndex"`
	} `json:"current_condition"`
	Weather []struct {
		Date     string `json:"date"`
		MaxTempC string `json:"maxtempC"`
		MinTempC string `json:"mintempC"`
	} `json:"weather"`
	NearestArea []struct {
		AreaName []struct {
			Value string `json:"value"`
		} `json:"areaName"`
		Country []struct {
			Value string `json:"value"`
		} `json:"country"`
		Region []struct {
			Value string `json:"value"`
		} `json:"region"`
	} `json:"nearest_area"`
}

// Current fetches the current weather for the given city.
func (c *Client) Current(ctx context.Context, city string) (*Weather, error) {
	encoded := url.PathEscape(city)
	u := fmt.Sprintf("%s/%s?format=j1", c.cfg.BaseURL, encoded)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var raw wttrJ1
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("wttr: decode response: %w", err)
	}
	w := &Weather{City: city}
	if len(raw.CurrentCondition) > 0 {
		cc := raw.CurrentCondition[0]
		w.TempC = cc.TempC
		w.TempF = cc.TempF
		w.FeelsLikeC = cc.FeelsLikeC
		w.WindSpeed = cc.WindspeedKmph
		w.Humidity = cc.Humidity
		w.Visibility = cc.Visibility
		w.Pressure = cc.Pressure
		w.UV = cc.UVIndex
		if len(cc.WeatherDesc) > 0 {
			w.Description = cc.WeatherDesc[0].Value
		}
	}
	if len(raw.Weather) > 0 {
		w.Date = raw.Weather[0].Date
		w.MaxTempC = raw.Weather[0].MaxTempC
		w.MinTempC = raw.Weather[0].MinTempC
	}
	if len(raw.NearestArea) > 0 {
		na := raw.NearestArea[0]
		if len(na.Country) > 0 {
			w.Country = na.Country[0].Value
		}
		if len(na.Region) > 0 {
			w.Region = na.Region[0].Value
		}
	}
	return w, nil
}

func (c *Client) get(ctx context.Context, u string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, u)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("wttr: get %s: %w", u, lastErr)
}

func (c *Client) do(ctx context.Context, u string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
