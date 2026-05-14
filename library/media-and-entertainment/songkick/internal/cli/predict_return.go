// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/songkick/internal/store"
)

type predictWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type predictResult struct {
	ArtistID           string         `json:"artist_id"`
	City               string         `json:"city"`
	Visits             int            `json:"visits"`
	MedianIntervalDays int            `json:"median_interval_days,omitempty"`
	StdevIntervalDays  int            `json:"stdev_interval_days,omitempty"`
	LastVisit          string         `json:"last_visit,omitempty"`
	NextProbableWindow *predictWindow `json:"next_probable_window,omitempty"`
	Confidence         string         `json:"confidence,omitempty"`
	Note               string         `json:"note,omitempty"`
}

func newPredictReturnCmd(flags *rootFlags) *cobra.Command {
	var artistID, city, dbPath string
	cmd := &cobra.Command{
		Use:   "predict-return",
		Short: "Forecast an artist's next probable visit window to a city",
		Example: strings.Trim(`
  songkick-pp-cli predict-return --artist 297938 --city jakarta --json
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
			cLat, cLng, ok := parseAnchor(city)
			if !ok {
				return usageErr(fmt.Errorf("unknown city %q", city))
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
			dates := make([]time.Time, 0)
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
				if greatCircleKm(cLat, cLng, lat, lng) > 2000 {
					continue
				}
				d, perr := time.Parse("2006-01-02", ev.DisplayDate)
				if perr != nil {
					continue
				}
				dates = append(dates, d)
			}
			sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })

			result := predictResult{ArtistID: artistID, City: city, Visits: len(dates)}
			if len(dates) < 2 {
				result.Note = "insufficient history"
				return flags.printJSON(cmd, result)
			}

			intervals := make([]float64, 0, len(dates)-1)
			for i := 1; i < len(dates); i++ {
				intervals = append(intervals, dates[i].Sub(dates[i-1]).Hours()/24.0)
			}
			med := medianFloat(intervals)
			sd := stdevFloat(intervals)
			last := dates[len(dates)-1]
			next := last.AddDate(0, 0, int(med))
			lo := next.AddDate(0, 0, -int(sd))
			hi := next.AddDate(0, 0, int(sd))

			result.MedianIntervalDays = int(math.Round(med))
			result.StdevIntervalDays = int(math.Round(sd))
			result.LastVisit = last.Format("2006-01-02")
			result.NextProbableWindow = &predictWindow{
				Start: lo.Format("2006-01-02"),
				End:   hi.Format("2006-01-02"),
			}
			switch {
			case len(dates) >= 5:
				result.Confidence = "high"
			case len(dates) >= 3:
				result.Confidence = "medium"
			default:
				result.Confidence = "low"
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&artistID, "artist", "", "Artist ID (required)")
	cmd.Flags().StringVar(&city, "city", "jakarta", "Target city")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func medianFloat(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64{}, xs...)
	sort.Float64s(cp)
	n := len(cp)
	if n%2 == 1 {
		return cp[n/2]
	}
	return (cp[n/2-1] + cp[n/2]) / 2.0
}

func stdevFloat(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	var ss float64
	for _, x := range xs {
		ss += (x - mean) * (x - mean)
	}
	return math.Sqrt(ss / float64(len(xs)-1))
}
