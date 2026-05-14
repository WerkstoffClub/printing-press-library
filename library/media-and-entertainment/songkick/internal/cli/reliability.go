// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/songkick/internal/store"
)

type reliabilityResult struct {
	ArtistID     string         `json:"artist_id"`
	Years        int            `json:"years"`
	TotalEvents  int            `json:"total_events"`
	ByStatus     map[string]int `json:"by_status"`
	CancelRate   float64        `json:"cancel_rate"`
	PostponeRate float64        `json:"postpone_rate"`
	Verdict      string         `json:"verdict"`
	Note         string         `json:"note,omitempty"`
}

func newReliabilityCmd(flags *rootFlags) *cobra.Command {
	var artistID, dbPath string
	var years int
	cmd := &cobra.Command{
		Use:   "reliability",
		Short: "Cancel/postpone rate per artist",
		Example: strings.Trim(`
  songkick-pp-cli reliability --artist 297938 --years 3 --json
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
			cutoff := time.Now().AddDate(-years, 0, 0)
			byStatus := map[string]int{"ok": 0, "cancelled": 0, "postponed": 0}
			total := 0
			for _, ev := range events {
				if !artistMatches(ev, wantID, artistID) {
					continue
				}
				d, perr := time.Parse("2006-01-02", ev.DisplayDate)
				if perr == nil && d.Before(cutoff) {
					continue
				}
				status := strings.ToLower(ev.Status)
				switch status {
				case "cancelled", "canceled":
					byStatus["cancelled"]++
				case "postponed":
					byStatus["postponed"]++
				default:
					byStatus["ok"]++
				}
				total++
			}
			result := reliabilityResult{
				ArtistID:    artistID,
				Years:       years,
				TotalEvents: total,
				ByStatus:    byStatus,
			}
			if total == 0 {
				result.Note = "no events synced for artist"
				result.Verdict = "unknown"
				return flags.printJSON(cmd, result)
			}
			result.CancelRate = roundTo(float64(byStatus["cancelled"])/float64(total), 4)
			result.PostponeRate = roundTo(float64(byStatus["postponed"])/float64(total), 4)
			switch {
			case result.CancelRate < 0.02:
				result.Verdict = "highly reliable"
			case result.CancelRate < 0.05:
				result.Verdict = "reliable"
			case result.CancelRate < 0.10:
				result.Verdict = "moderate"
			default:
				result.Verdict = "high risk"
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&artistID, "artist", "", "Artist ID (required)")
	cmd.Flags().IntVar(&years, "years", 3, "Years back to consider")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func roundTo(x float64, decimals int) float64 {
	mul := 1.0
	for i := 0; i < decimals; i++ {
		mul *= 10
	}
	return float64(int64(x*mul+0.5)) / mul
}
