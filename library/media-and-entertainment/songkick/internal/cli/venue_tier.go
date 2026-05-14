// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/songkick/internal/store"
)

type venueTierEntry struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"displayName"`
	Capacity    int    `json:"capacity"`
	Tier        string `json:"tier"`
}

type venueTierResult struct {
	MetroID string           `json:"metro_id"`
	Venues  []venueTierEntry `json:"venues"`
	Note    string           `json:"note,omitempty"`
}

func newVenueTierCmd(flags *rootFlags) *cobra.Command {
	var metroID, dbPath string
	cmd := &cobra.Command{
		Use:   "venue-tier",
		Short: "Tier-classify venues in a metro",
		Example: strings.Trim(`
  songkick-pp-cli venue-tier --metro 17681 --json
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
			if metroID == "" {
				return usageErr(fmt.Errorf("--metro is required"))
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
			venueMap := map[int64]venueTierEntry{}
			caps := map[int64]int{}
			for _, ev := range events {
				if ev.MetroAreaID != metroN {
					continue
				}
				if ev.Venue.ID == 0 {
					continue
				}
				if _, seen := venueMap[ev.Venue.ID]; !seen {
					venueMap[ev.Venue.ID] = venueTierEntry{
						ID:          ev.Venue.ID,
						DisplayName: ev.Venue.DisplayName,
						Capacity:    ev.Venue.Capacity,
					}
					caps[ev.Venue.ID] = ev.Venue.Capacity
				}
			}
			tiers := assignVenueTiers(caps)
			out := make([]venueTierEntry, 0, len(venueMap))
			for id, v := range venueMap {
				v.Tier = tiers[id]
				if v.Tier == "" {
					v.Tier = "C"
				}
				out = append(out, v)
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].Capacity != out[j].Capacity {
					return out[i].Capacity > out[j].Capacity
				}
				return out[i].DisplayName < out[j].DisplayName
			})
			result := venueTierResult{MetroID: metroID, Venues: out}
			if len(out) == 0 {
				result.Note = "no venues found for metro"
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&metroID, "metro", "", "Metro area ID (required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
