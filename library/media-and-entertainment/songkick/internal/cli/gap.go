// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/songkick/internal/store"
)

type gapEntry struct {
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
	DaysOpen      int    `json:"days_open"`
	PrecedingCity string `json:"preceding_city"`
	FollowingCity string `json:"following_city"`
}

type gapResult struct {
	ArtistID string     `json:"artist_id"`
	Anchor   string     `json:"anchor"`
	Window   int        `json:"window"`
	Gaps     []gapEntry `json:"gaps"`
	Note     string     `json:"note,omitempty"`
}

func newGapCmd(flags *rootFlags) *cobra.Command {
	var artistID, anchor, dbPath string
	var window int
	var radius float64
	cmd := &cobra.Command{
		Use:   "gap",
		Short: "Find date windows where artist is in region but not at anchor",
		Example: strings.Trim(`
  songkick-pp-cli gap --artist 297938 --anchor jakarta --window 21 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("artist") {
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
			aLat, aLng, ok := parseAnchor(anchor)
			if !ok {
				return usageErr(fmt.Errorf("unknown anchor %q", anchor))
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

			wantID := asInt64(artistID)
			type stop struct {
				date time.Time
				city string
			}
			stops := make([]stop, 0)
			for _, ev := range events {
				if !artistMatches(ev, wantID, artistID) {
					continue
				}
				lat, lng := ev.Venue.Lat, ev.Venue.Lng
				if lat == 0 && lng == 0 {
					lat, lng = ev.Location.Lat, ev.Location.Lng
				}
				if lat == 0 && lng == 0 {
					continue
				}
				if greatCircleKm(aLat, aLng, lat, lng) > radius {
					continue
				}
				d, perr := time.Parse("2006-01-02", ev.DisplayDate)
				if perr != nil {
					continue
				}
				stops = append(stops, stop{date: d, city: ev.Location.City})
			}
			sort.Slice(stops, func(i, j int) bool { return stops[i].date.Before(stops[j].date) })

			result := gapResult{ArtistID: artistID, Anchor: anchor, Window: window, Gaps: []gapEntry{}}
			if len(stops) < 2 {
				result.Note = "insufficient regional history"
				return flags.printJSON(cmd, result)
			}
			for i := 1; i < len(stops); i++ {
				diff := int(stops[i].date.Sub(stops[i-1].date).Hours() / 24)
				if diff < 3 || diff > window {
					continue
				}
				result.Gaps = append(result.Gaps, gapEntry{
					StartDate:     stops[i-1].date.Format("2006-01-02"),
					EndDate:       stops[i].date.Format("2006-01-02"),
					DaysOpen:      diff,
					PrecedingCity: stops[i-1].city,
					FollowingCity: stops[i].city,
				})
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&artistID, "artist", "", "Artist ID (required)")
	cmd.Flags().StringVar(&anchor, "anchor", "jakarta", "Anchor city")
	cmd.Flags().IntVar(&window, "window", 21, "Max days for an open gap")
	cmd.Flags().Float64Var(&radius, "radius", 5000, "Max distance from anchor in km")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
