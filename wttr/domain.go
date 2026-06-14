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
			Short:  "Get current weather for any city.",
			Long: `wttr fetches real-time weather conditions for any city from wttr.in.

It returns temperature, feels-like, wind speed, humidity, UV index, and today's
forecast range in a clean JSON record. No API key required.`,
			Site: Host,
			Repo: "https://github.com/tamnd/wttr-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:     "weather",
		Group:    "read",
		Single:   true,
		Summary:  "Get current weather for a city",
		URIType:  "city",
		Resolver: true,
		Args:     []kit.Arg{{Name: "city", Help: "city name (e.g. Tokyo, New+York, Paris)"}},
	}, getWeather)
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

type weatherInput struct {
	City   string  `kit:"arg" help:"city name"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func getWeather(ctx context.Context, in weatherInput, emit func(*Weather) error) error {
	w, err := in.Client.Current(ctx, in.City)
	if err != nil {
		return mapErr(err)
	}
	return emit(w)
}

// --- Resolver ---

// Classify turns a city name or wttr.in URL into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if u, err2 := url.Parse(input); err2 == nil && (u.Scheme == "http" || u.Scheme == "https") {
		city := strings.Trim(u.Path, "/")
		if city != "" {
			return "city", city, nil
		}
	}
	if input == "" {
		return "", "", errs.Usage("unrecognized wttr reference: %q", input)
	}
	return "city", input, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	if uriType != "city" {
		return "", errs.Usage("wttr has no resource type %q", uriType)
	}
	return "https://" + Host + "/" + url.PathEscape(id) + "?format=j1", nil
}

func mapErr(err error) error {
	return err
}
