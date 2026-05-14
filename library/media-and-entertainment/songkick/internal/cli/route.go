// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/songkick/internal/store"
)

type routeResult struct {
	ArtistID    string           `json:"artist_id"`
	Anchor      string           `json:"anchor"`
	On          string           `json:"on"`
	Window      int              `json:"window"`
	Radius      float64          `json:"radius"`
	NearestShow *routeNearest    `json:"nearest_show"`
	Score       float64          `json:"score"`
	Verdict     string           `json:"verdict"`
	Note        string           `json:"note,omitempty"`
	Candidates  []routeCandidate `json:"candidates,omitempty"`
}

type routeNearest struct {
	Date           string  `json:"date"`
	City           string  `json:"city"`
	Venue          string  `json:"venue"`
	DistanceKm     float64 `json:"distanceKm"`
	DaysFromTarget int     `json:"daysFromTarget"`
}

type routeCandidate struct {
	Date           string  `json:"date"`
	City           string  `json:"city"`
	Venue          string  `json:"venue"`
	DistanceKm     float64 `json:"distanceKm"`
	DaysFromTarget int     `json:"daysFromTarget"`
	Score          float64 `json:"score"`
}

func newRouteCmd(flags *rootFlags) *cobra.Command {
	var artistID, anchor, on, dbPath string
	var window int
	var radius float64
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Score one artist for routing feasibility against a target date and anchor city",
		Example: strings.Trim(`
  songkick-pp-cli route --artist 297938 --anchor jakarta --on 2026-09-20 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("artist") && !cmd.Flags().Changed("on") {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if artistID == "" {
				return usageErr(fmt.Errorf("--artist is required"))
			}
			if on == "" {
				return usageErr(fmt.Errorf("--on (YYYY-MM-DD) is required"))
			}
			target, err := time.Parse("2006-01-02", on)
			if err != nil {
				return usageErr(fmt.Errorf("--on must be YYYY-MM-DD: %w", err))
			}
			aLat, aLng, ok := parseAnchor(anchor)
			if !ok {
				return usageErr(fmt.Errorf("unknown anchor %q (try jakarta, singapore, tokyo, sydney, london, newyork)", anchor))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("songkick-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			events, err := loadStoredEvents(cmd.Context(), db)
			if err != nil {
				return err
			}

			result := routeResult{
				ArtistID: artistID,
				Anchor:   anchor,
				On:       on,
				Window:   window,
				Radius:   radius,
				Score:    0,
				Verdict:  "cold",
			}

			candidates := scoreCandidatesForArtist(events, artistID, target, aLat, aLng, window, radius)
			if len(candidates) == 0 {
				result.Note = "no events synced for artist"
				return flags.printJSON(cmd, result)
			}

			best := candidates[0]
			for _, c := range candidates[1:] {
				if c.Score > best.Score {
					best = c
				}
			}
			result.NearestShow = &routeNearest{
				Date:           best.Date,
				City:           best.City,
				Venue:          best.Venue,
				DistanceKm:     best.DistanceKm,
				DaysFromTarget: best.DaysFromTarget,
			}
			result.Score = math.Round(best.Score*10) / 10
			result.Verdict = verdictFor(best.Score)
			result.Candidates = candidates
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&artistID, "artist", "", "Artist ID (required)")
	cmd.Flags().StringVar(&anchor, "anchor", "jakarta", "Anchor city")
	cmd.Flags().StringVar(&on, "on", "", "Target date YYYY-MM-DD (required)")
	cmd.Flags().IntVar(&window, "window", 14, "Days tolerance around target")
	cmd.Flags().Float64Var(&radius, "radius", 5000, "Max distance from anchor in km")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func scoreCandidatesForArtist(events []storedEvent, artistID string, target time.Time, aLat, aLng float64, window int, radius float64) []routeCandidate {
	out := make([]routeCandidate, 0)
	wantID := asInt64(artistID)
	for _, ev := range events {
		if !artistMatches(ev, wantID, artistID) {
			continue
		}
		if ev.DisplayDate == "" {
			continue
		}
		d, err := time.Parse("2006-01-02", ev.DisplayDate)
		if err != nil {
			continue
		}
		days := int(d.Sub(target).Hours() / 24)
		absDays := days
		if absDays < 0 {
			absDays = -absDays
		}
		lat, lng := ev.Venue.Lat, ev.Venue.Lng
		if lat == 0 && lng == 0 {
			lat, lng = ev.Location.Lat, ev.Location.Lng
		}
		if lat == 0 && lng == 0 {
			continue
		}
		dist := greatCircleKm(aLat, aLng, lat, lng)
		if absDays > window || dist > radius {
			continue
		}
		score := 100.0 - 60.0*float64(absDays)/float64(window) - 40.0*dist/radius
		if score < 0 {
			score = 0
		}
		if score > 100 {
			score = 100
		}
		out = append(out, routeCandidate{
			Date:           ev.DisplayDate,
			City:           ev.Location.City,
			Venue:          ev.Venue.DisplayName,
			DistanceKm:     math.Round(dist*10) / 10,
			DaysFromTarget: days,
			Score:          math.Round(score*10) / 10,
		})
	}
	return out
}

func artistMatches(ev storedEvent, wantID int64, artistID string) bool {
	for _, p := range ev.Performances {
		if wantID != 0 && p.ArtistID == wantID {
			return true
		}
		if artistID != "" && fmt.Sprintf("%d", p.ArtistID) == artistID {
			return true
		}
	}
	return false
}

func verdictFor(score float64) string {
	switch {
	case score >= 80:
		return "strong-fit"
	case score >= 40:
		return "considered"
	default:
		return "cold"
	}
}
