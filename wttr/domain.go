package wttr

import (
	"context"
	"net/url"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the wttr driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against, and
// the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "wttr",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "wttr",
			Short:  "Get weather for any location from wttr.in.",
			Long: `wttr fetches weather from wttr.in for any city, airport code, or lat/lon.

No API key required. Use "current" for live conditions or "forecast" for the
3-day outlook.`,
			Site: Host,
			Repo: "https://github.com/tamnd/wttr-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:     "current",
		Group:    "read",
		Single:   true,
		Summary:  "Get current weather for a location",
		URIType:  "location",
		Resolver: true,
		Args:     []kit.Arg{{Name: "location", Help: "city name, airport code, or lat,lon (e.g. Paris, JFK, 48.8566,2.3522)"}},
	}, getCurrent)

	kit.Handle(app, kit.OpMeta{
		Name:    "forecast",
		Group:   "read",
		Single:  false,
		Summary: "Get 3-day weather forecast for a location",
		URIType: "location",
		Args:    []kit.Arg{{Name: "location", Help: "city name, airport code, or lat,lon"}},
	}, getForecast)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type currentInput struct {
	Location string  `kit:"arg" help:"city name, airport code, or lat,lon"`
	Client   *Client `kit:"inject"`
}

type forecastInput struct {
	Location string  `kit:"arg" help:"city name, airport code, or lat,lon"`
	Client   *Client `kit:"inject"`
}

// --- handlers ---

func getCurrent(ctx context.Context, in currentInput, emit func(*Current) error) error {
	w, err := in.Client.Current(ctx, in.Location)
	if err != nil {
		return mapErr(err)
	}
	return emit(w)
}

func getForecast(ctx context.Context, in forecastInput, emit func(*ForecastDay) error) error {
	days, err := in.Client.Forecast(ctx, in.Location)
	if err != nil {
		return mapErr(err)
	}
	for _, d := range days {
		if err := emit(d); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver ---

// Classify turns a location string or wttr.in URL into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if u, err2 := url.Parse(input); err2 == nil && (u.Scheme == "http" || u.Scheme == "https") {
		loc := strings.Trim(u.Path, "/")
		if loc != "" {
			return "location", loc, nil
		}
	}
	if input == "" {
		return "", "", errs.Usage("unrecognized wttr reference: %q", input)
	}
	return "location", input, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	if uriType != "location" {
		return "", errs.Usage("wttr has no resource type %q", uriType)
	}
	return "https://" + Host + "/" + url.PathEscape(id) + "?format=j1", nil
}

func mapErr(err error) error {
	return err
}
