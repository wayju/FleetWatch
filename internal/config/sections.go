package config

// The cost, models and recap sections (see ADR-005). They carry what used to be
// compiled in: the price file, the model table and the recap gateway and
// model. Forges are configured by the forges list (see ADR-005). Precedence is the same as everywhere: environment,
// then file, then a neutral default (nothing configured, feature off).
// FLEET_RECAP_KEY and GITEA_TOKEN stay in the environment; no struct here has
// a field for a secret.

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wayju/fleetwatch/internal/models"
)

type costSection struct {
	Prices *string `json:"prices"`
}

type recapSection struct {
	URL   *string `json:"url"`
	Model *string `json:"model"`
	Every *string `json:"every"`
	// AllowInsecure admits a plain-http url to a non-loopback host.
	AllowInsecure bool `json:"allow_insecure"`
}

func (r *Resolved) sections(env Env, doc *fileDoc) error {
	if doc == nil {
		doc = &fileDoc{}
	}
	var cost costSection
	var recap recapSection
	var entries []models.Entry
	for _, s := range []struct {
		name string
		raw  []byte
		into any
	}{{"cost", doc.Cost, &cost}, {"recap", doc.Recap, &recap}, {"models", doc.Models, &entries}} {
		if !present(s.raw) {
			continue
		}
		if err := strict(s.raw, s.into); err != nil {
			return fmt.Errorf("config %s: %s: %v", r.Path, s.name, err)
		}
	}
	tbl, err := models.New(entries)
	if err != nil {
		return fmt.Errorf("config %s: %v", r.Path, err)
	}
	r.Models = tbl
	r.Sources["models"] = "default"
	if len(entries) > 0 {
		r.Sources["models"] = "file"
	}
	r.pickStr("cost.prices", env.Get("FLEET_PRICES_FILE"), cost.Prices, &r.PricesFile)
	r.PricesFile = r.expand(r.PricesFile)
	r.pickStr("recap.url", env.Get("FLEET_RECAP_URL"), recap.URL, &r.RecapURL)
	r.pickStr("recap.model", env.Get("FLEET_RECAP_MODEL"), recap.Model, &r.RecapModel)
	if r.RecapURL != "" {
		if err := r.checkTransport("recap.url", r.RecapURL, recap.AllowInsecure); err != nil {
			return fmt.Errorf("config %s: %v", r.Path, err)
		}
	}
	return r.recapEvery(env.Get("FLEET_RECAP_EVERY"), recap.Every)
}

func (r *Resolved) pickStr(key, envVal string, fileVal *string, to *string) {
	r.Sources[key] = "default"
	switch {
	case envVal != "":
		*to, r.Sources[key] = envVal, "env"
	case fileVal != nil:
		*to, r.Sources[key] = *fileVal, "file"
	}
}

func (r *Resolved) pickList(key, envVal string, fileVal []string, to *[]string) {
	r.Sources[key] = "default"
	switch {
	case envVal != "":
		*to, r.Sources[key] = lowerAll(strings.Split(envVal, ",")), "env"
	case fileVal != nil:
		*to, r.Sources[key] = lowerAll(fileVal), "file"
	}
}

func (r *Resolved) recapEvery(envVal string, fileVal *string) error {
	r.RecapEvery, r.Sources["recap.every"] = 5*time.Minute, "default"
	if fileVal != nil {
		v, err := time.ParseDuration(*fileVal)
		if err != nil || v <= 0 {
			return fmt.Errorf("config %s: recap.every must be a positive duration like \"5m\", got %q", r.Path, *fileVal)
		}
		r.RecapEvery, r.Sources["recap.every"] = v, "file"
	}
	if v, err := time.ParseDuration(envVal); err == nil && v > 0 {
		r.RecapEvery, r.Sources["recap.every"] = v, "env"
	}
	return nil
}

// sectionValue renders a section setting for Describe.
func (r *Resolved) sectionValue(key string) (string, bool) {
	if v, ok := r.uiValue(key); ok {
		return v, true
	}
	switch key {
	case "cost.prices":
		return r.PricesFile, true
	case "recap.url":
		return r.RecapURL, true
	case "recap.model":
		return r.RecapModel, true
	case "recap.every":
		return r.RecapEvery.String(), true
	case "models":
		return strconv.Itoa(r.Models.Len()) + " entries", true
	}
	return "", false
}
