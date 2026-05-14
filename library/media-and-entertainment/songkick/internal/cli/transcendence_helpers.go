// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/songkick/internal/store"
)

// greatCircleKm returns the great-circle distance between two lat/lng pairs in km.
func greatCircleKm(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371.0
	toRad := func(d float64) float64 { return d * math.Pi / 180.0 }
	phi1 := toRad(lat1)
	phi2 := toRad(lat2)
	dPhi := toRad(lat2 - lat1)
	dLam := toRad(lng2 - lng1)
	a := math.Sin(dPhi/2)*math.Sin(dPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(dLam/2)*math.Sin(dLam/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// anchorCoords maps a small set of well-known anchor city names to lat/lng.
var anchorCoords = map[string][2]float64{
	"jakarta":   {-6.2088, 106.8456},
	"singapore": {1.3521, 103.8198},
	"tokyo":     {35.6762, 139.6503},
	"sydney":    {-33.8688, 151.2093},
	"bangkok":   {13.7563, 100.5018},
	"manila":    {14.5995, 120.9842},
	"seoul":     {37.5665, 126.978},
	"hongkong":  {22.3193, 114.1694},
	"taipei":    {25.033, 121.5654},
	"london":    {51.5074, -0.1278},
	"newyork":   {40.7128, -74.006},
	"paris":     {48.8566, 2.3522},
}

func parseAnchor(name string) (lat, lng float64, ok bool) {
	key := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", ""))
	c, found := anchorCoords[key]
	if !found {
		return 0, 0, false
	}
	return c[0], c[1], true
}

// storedVenue captures the relevant venue fields from a stored event.
type storedVenue struct {
	ID          int64   `json:"id"`
	DisplayName string  `json:"displayName"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	Capacity    int     `json:"capacity"`
}

type storedPerformance struct {
	ID           int64  `json:"id"`
	DisplayName  string `json:"displayName"`
	BillingIndex int    `json:"billingIndex"`
	BillingOrder string `json:"billingOrder"`
	ArtistID     int64  `json:"artistID"`
}

type storedLocation struct {
	City    string  `json:"city"`
	Country string  `json:"country"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
}

type storedEvent struct {
	ID           int64               `json:"id"`
	DisplayName  string              `json:"displayName"`
	Status       string              `json:"status"`
	Type         string              `json:"type"`
	DisplayDate  string              `json:"displayDate"`
	Popularity   float64             `json:"popularity"`
	Venue        storedVenue         `json:"venue"`
	Performances []storedPerformance `json:"performances"`
	Location     storedLocation      `json:"location"`
	MetroAreaID  int64               `json:"metroAreaID"`
}

// loadStoredEvents loads every event row and parses tolerant of missing fields.
func loadStoredEvents(_ context.Context, db *store.Store) ([]storedEvent, error) {
	raws, err := db.List("events", 100000)
	if err != nil {
		return nil, fmt.Errorf("listing events: %w", err)
	}
	out := make([]storedEvent, 0, len(raws))
	for _, raw := range raws {
		ev, ok := parseStoredEvent(raw)
		if !ok {
			continue
		}
		out = append(out, ev)
	}
	return out, nil
}

func parseStoredEvent(raw json.RawMessage) (storedEvent, bool) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return storedEvent{}, false
	}
	ev := storedEvent{}
	ev.ID = asInt64(obj["id"])
	ev.DisplayName = asString(obj["displayName"])
	ev.Status = asString(obj["status"])
	ev.Type = asString(obj["type"])
	ev.Popularity = asFloat(obj["popularity"])

	if startObj, ok := obj["start"].(map[string]any); ok {
		ev.DisplayDate = asString(startObj["date"])
	}
	if ev.DisplayDate == "" {
		// some events expose start.date or just "displayDate" or "displayedDate"
		if d := asString(obj["displayDate"]); d != "" {
			ev.DisplayDate = d
		}
	}

	if v, ok := obj["venue"].(map[string]any); ok {
		ev.Venue.ID = asInt64(v["id"])
		ev.Venue.DisplayName = asString(v["displayName"])
		ev.Venue.Lat = asFloat(v["lat"])
		ev.Venue.Lng = asFloat(v["lng"])
		ev.Venue.Capacity = int(asInt64(v["capacity"]))
		// metroArea may live on venue
		if m, ok := v["metroArea"].(map[string]any); ok {
			if ev.MetroAreaID == 0 {
				ev.MetroAreaID = asInt64(m["id"])
			}
		}
	}

	if loc, ok := obj["location"].(map[string]any); ok {
		ev.Location.City = asString(loc["city"])
		ev.Location.Country = asString(loc["country"])
		ev.Location.Lat = asFloat(loc["lat"])
		ev.Location.Lng = asFloat(loc["lng"])
		// fall back venue lat/lng if missing
		if ev.Venue.Lat == 0 && ev.Venue.Lng == 0 {
			ev.Venue.Lat = ev.Location.Lat
			ev.Venue.Lng = ev.Location.Lng
		}
	}

	if m, ok := obj["metroArea"].(map[string]any); ok {
		if ev.MetroAreaID == 0 {
			ev.MetroAreaID = asInt64(m["id"])
		}
	}

	if perfs, ok := obj["performance"].([]any); ok {
		for _, p := range perfs {
			pm, ok := p.(map[string]any)
			if !ok {
				continue
			}
			sp := storedPerformance{
				ID:           asInt64(pm["id"]),
				DisplayName:  asString(pm["displayName"]),
				BillingIndex: int(asInt64(pm["billingIndex"])),
				BillingOrder: asString(pm["billingOrder"]),
			}
			if a, ok := pm["artist"].(map[string]any); ok {
				sp.ArtistID = asInt64(a["id"])
			}
			ev.Performances = append(ev.Performances, sp)
		}
	}
	if len(ev.Performances) == 0 {
		if perfs, ok := obj["performances"].([]any); ok {
			for _, p := range perfs {
				pm, ok := p.(map[string]any)
				if !ok {
					continue
				}
				sp := storedPerformance{
					ID:           asInt64(pm["id"]),
					DisplayName:  asString(pm["displayName"]),
					BillingIndex: int(asInt64(pm["billingIndex"])),
					BillingOrder: asString(pm["billingOrder"]),
				}
				if a, ok := pm["artist"].(map[string]any); ok {
					sp.ArtistID = asInt64(a["id"])
				}
				ev.Performances = append(ev.Performances, sp)
			}
		}
	}

	return ev, true
}

func asInt64(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	case string:
		var n int64
		_, _ = fmt.Sscanf(x, "%d", &n)
		return n
	}
	return 0
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int64:
		return float64(x)
	case int:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		var n float64
		_, _ = fmt.Sscanf(x, "%f", &n)
		return n
	}
	return 0
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprintf("%g", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case int:
		return fmt.Sprintf("%d", x)
	case bool:
		return fmt.Sprintf("%t", x)
	}
	return ""
}
