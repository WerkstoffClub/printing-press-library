// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/songkick/internal/store"
)

type promotionEntry struct {
	TransitionDate string `json:"transition_date"`
	FromBilling    int    `json:"from_billing"`
	ToBilling      int    `json:"to_billing"`
	EventID        int64  `json:"event_id"`
	DisplayName    string `json:"displayName"`
}

type promotionResult struct {
	ArtistID   string           `json:"artist_id"`
	Promotions []promotionEntry `json:"promotions"`
	Note       string           `json:"note,omitempty"`
}

func newPromotionCmd(flags *rootFlags) *cobra.Command {
	var artistID, dbPath string
	cmd := &cobra.Command{
		Use:   "promotion",
		Short: "Detect billing-index inflection points for an artist",
		Example: strings.Trim(`
  songkick-pp-cli promotion --artist 297938 --json
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
			type point struct {
				date    string
				billing int
				eventID int64
				display string
			}
			pts := make([]point, 0)
			for _, ev := range events {
				for _, p := range ev.Performances {
					if (wantID != 0 && p.ArtistID == wantID) ||
						(artistID != "" && fmt.Sprintf("%d", p.ArtistID) == artistID) {
						pts = append(pts, point{
							date:    ev.DisplayDate,
							billing: p.BillingIndex,
							eventID: ev.ID,
							display: ev.DisplayName,
						})
						break
					}
				}
			}
			sort.Slice(pts, func(i, j int) bool { return pts[i].date < pts[j].date })

			result := promotionResult{ArtistID: artistID, Promotions: []promotionEntry{}}
			prev := -1
			for _, p := range pts {
				if prev > 0 && p.billing == 0 {
					result.Promotions = append(result.Promotions, promotionEntry{
						TransitionDate: p.date,
						FromBilling:    prev,
						ToBilling:      p.billing,
						EventID:        p.eventID,
						DisplayName:    p.display,
					})
				}
				prev = p.billing
			}
			if len(pts) == 0 {
				result.Note = "no events found for artist"
			} else if len(result.Promotions) == 0 {
				result.Note = "no promotion transitions detected"
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&artistID, "artist", "", "Artist ID (required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
