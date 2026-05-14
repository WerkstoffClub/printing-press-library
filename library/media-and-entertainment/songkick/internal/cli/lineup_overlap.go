// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/songkick/internal/store"
)

type sharedArtist struct {
	ArtistID            int64          `json:"artist_id"`
	DisplayName         string         `json:"displayName"`
	BillingPerEvent     map[string]int `json:"billing_per_event"`
	PromotedToHeadliner *string        `json:"promoted_to_headliner_in"`
}

type overlapResult struct {
	Events        []int64        `json:"events"`
	SharedArtists []sharedArtist `json:"shared_artists"`
	OverlapPct    float64        `json:"overlap_pct"`
	Note          string         `json:"note,omitempty"`
}

func newLineupOverlapCmd(flags *rootFlags) *cobra.Command {
	var eventsCSV, dbPath string
	cmd := &cobra.Command{
		Use:   "lineup-overlap",
		Short: "Compare performances across festival event IDs",
		Example: strings.Trim(`
  songkick-pp-cli lineup-overlap --events 12345,67890 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("events") {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if eventsCSV == "" {
				return usageErr(fmt.Errorf("--events is required (csv of event IDs)"))
			}
			ids := []int64{}
			for _, raw := range strings.Split(eventsCSV, ",") {
				raw = strings.TrimSpace(raw)
				if raw == "" {
					continue
				}
				n := asInt64(raw)
				if n != 0 {
					ids = append(ids, n)
				}
			}
			if len(ids) < 2 {
				return usageErr(fmt.Errorf("provide at least 2 event IDs"))
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
			byID := map[int64]storedEvent{}
			for _, ev := range events {
				byID[ev.ID] = ev
			}
			result := overlapResult{Events: ids, SharedArtists: []sharedArtist{}}
			missing := []int64{}
			perfsByEvent := map[int64][]storedPerformance{}
			for _, id := range ids {
				ev, ok := byID[id]
				if !ok {
					missing = append(missing, id)
					continue
				}
				perfsByEvent[id] = ev.Performances
			}
			if len(missing) == len(ids) {
				result.Note = "events not found in local store"
				return flags.printJSON(cmd, result)
			}

			counts := map[int64]int{}
			names := map[int64]string{}
			billings := map[int64]map[string]int{}
			for evID, perfs := range perfsByEvent {
				for _, p := range perfs {
					if p.ArtistID == 0 {
						continue
					}
					counts[p.ArtistID]++
					if names[p.ArtistID] == "" {
						names[p.ArtistID] = p.DisplayName
					}
					if billings[p.ArtistID] == nil {
						billings[p.ArtistID] = map[string]int{}
					}
					billings[p.ArtistID][fmt.Sprintf("%d", evID)] = p.BillingIndex
				}
			}
			totalUnique := len(counts)
			for aid, n := range counts {
				if n < 2 {
					continue
				}
				sa := sharedArtist{
					ArtistID:        aid,
					DisplayName:     names[aid],
					BillingPerEvent: billings[aid],
				}
				// detect promotion to billing 0
				var promotedIn *string
				prev := -1
				for _, evID := range ids {
					bi, ok := billings[aid][fmt.Sprintf("%d", evID)]
					if !ok {
						continue
					}
					if prev > 0 && bi == 0 {
						s := fmt.Sprintf("%d", evID)
						promotedIn = &s
					}
					prev = bi
				}
				sa.PromotedToHeadliner = promotedIn
				result.SharedArtists = append(result.SharedArtists, sa)
			}
			sort.Slice(result.SharedArtists, func(i, j int) bool {
				return result.SharedArtists[i].DisplayName < result.SharedArtists[j].DisplayName
			})
			if totalUnique > 0 {
				result.OverlapPct = float64(len(result.SharedArtists)) / float64(totalUnique)
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&eventsCSV, "events", "", "CSV of event IDs (required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
