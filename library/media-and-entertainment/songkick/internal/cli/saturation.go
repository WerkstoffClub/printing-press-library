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

type saturationWeek struct {
	Week          string         `json:"week"`
	EventCount    int            `json:"event_count"`
	TotalCapacity int            `json:"total_capacity"`
	ByTier        map[string]int `json:"by_tier"`
}

type saturationResult struct {
	MetroID string           `json:"metro_id"`
	From    string           `json:"from"`
	To      string           `json:"to"`
	Weeks   []saturationWeek `json:"weeks"`
	Note    string           `json:"note,omitempty"`
}

func newSaturationCmd(flags *rootFlags) *cobra.Command {
	var metroID, from, to, dbPath string
	cmd := &cobra.Command{
		Use:   "saturation",
		Short: "Per-week show density for a metro",
		Example: strings.Trim(`
  songkick-pp-cli saturation --metro 17681 --from 2026-09-01 --to 2026-10-15 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("metro") {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if metroID == "" || from == "" || to == "" {
				return usageErr(fmt.Errorf("--metro, --from, --to are required"))
			}
			fromT, err := time.Parse("2006-01-02", from)
			if err != nil {
				return usageErr(fmt.Errorf("--from must be YYYY-MM-DD: %w", err))
			}
			toT, err := time.Parse("2006-01-02", to)
			if err != nil {
				return usageErr(fmt.Errorf("--to must be YYYY-MM-DD: %w", err))
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
			metroN := asInt64(metroID)

			venueCaps := map[int64]int{}
			for _, ev := range events {
				if ev.Venue.ID != 0 && ev.Venue.Capacity > 0 {
					venueCaps[ev.Venue.ID] = ev.Venue.Capacity
				}
			}
			tiers := assignVenueTiers(venueCaps)

			weeks := map[string]*saturationWeek{}
			for _, ev := range events {
				if ev.MetroAreaID != metroN {
					continue
				}
				if ev.DisplayDate == "" {
					continue
				}
				d, perr := time.Parse("2006-01-02", ev.DisplayDate)
				if perr != nil {
					continue
				}
				if d.Before(fromT) || d.After(toT) {
					continue
				}
				wy, w := d.ISOWeek()
				key := fmt.Sprintf("%d-W%02d", wy, w)
				wk, ok := weeks[key]
				if !ok {
					wk = &saturationWeek{
						Week:   key,
						ByTier: map[string]int{"S": 0, "A": 0, "B": 0, "C": 0, "unknown": 0},
					}
					weeks[key] = wk
				}
				wk.EventCount++
				wk.TotalCapacity += ev.Venue.Capacity
				tier := tiers[ev.Venue.ID]
				if tier == "" {
					tier = "unknown"
				}
				wk.ByTier[tier]++
			}

			result := saturationResult{
				MetroID: metroID,
				From:    from,
				To:      to,
				Weeks:   []saturationWeek{},
			}
			keys := make([]string, 0, len(weeks))
			for k := range weeks {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				result.Weeks = append(result.Weeks, *weeks[k])
			}
			if len(result.Weeks) == 0 {
				result.Note = "no events for metro in range"
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&metroID, "metro", "", "Metro area ID (required)")
	cmd.Flags().StringVar(&from, "from", "", "From date YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&to, "to", "", "To date YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func assignVenueTiers(caps map[int64]int) map[int64]string {
	out := map[int64]string{}
	type entry struct {
		id  int64
		cap int
	}
	xs := make([]entry, 0, len(caps))
	for id, c := range caps {
		xs = append(xs, entry{id, c})
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i].cap > xs[j].cap })
	n := len(xs)
	if n == 0 {
		return out
	}
	sCut := int(float64(n) * 0.10)
	aCut := sCut + int(float64(n)*0.20)
	bCut := aCut + int(float64(n)*0.30)
	for i, e := range xs {
		switch {
		case i < sCut:
			out[e.id] = "S"
		case i < aCut:
			out[e.id] = "A"
		case i < bCut:
			out[e.id] = "B"
		default:
			out[e.id] = "C"
		}
	}
	return out
}
