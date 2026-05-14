// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/songkick/internal/store"
)

type coArtistEntry struct {
	ArtistID    int64  `json:"artist_id"`
	DisplayName string `json:"displayName"`
	SharedBills int    `json:"shared_bills"`
	Depth       int    `json:"depth"`
}

type coTourResult struct {
	ArtistID  string          `json:"artist_id"`
	Depth     int             `json:"depth"`
	CoArtists []coArtistEntry `json:"co_artists"`
	Note      string          `json:"note,omitempty"`
}

func newCoTourCmd(flags *rootFlags) *cobra.Command {
	var artistID, dbPath string
	var depth int
	cmd := &cobra.Command{
		Use:   "co-tour",
		Short: "Artist graph from shared performances",
		Example: strings.Trim(`
  songkick-pp-cli co-tour --artist 297938 --depth 1 --json
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

			rootID := asInt64(artistID)
			seen := map[int64]int{rootID: 0}
			frontier := map[int64]bool{rootID: true}
			counts := map[int64]int{}
			names := map[int64]string{}

			for d := 1; d <= depth; d++ {
				nextFrontier := map[int64]bool{}
				for _, ev := range events {
					hit := false
					for _, p := range ev.Performances {
						if frontier[p.ArtistID] {
							hit = true
							break
						}
					}
					if !hit {
						continue
					}
					for _, p := range ev.Performances {
						if p.ArtistID == 0 || p.ArtistID == rootID {
							continue
						}
						if _, ok := seen[p.ArtistID]; !ok {
							seen[p.ArtistID] = d
							nextFrontier[p.ArtistID] = true
						}
						if seen[p.ArtistID] == d {
							counts[p.ArtistID]++
							if names[p.ArtistID] == "" {
								names[p.ArtistID] = p.DisplayName
							}
						}
					}
				}
				frontier = nextFrontier
				if len(frontier) == 0 {
					break
				}
			}

			out := make([]coArtistEntry, 0, len(counts))
			for aid, c := range counts {
				out = append(out, coArtistEntry{
					ArtistID:    aid,
					DisplayName: names[aid],
					SharedBills: c,
					Depth:       seen[aid],
				})
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].SharedBills != out[j].SharedBills {
					return out[i].SharedBills > out[j].SharedBills
				}
				return out[i].DisplayName < out[j].DisplayName
			})
			result := coTourResult{ArtistID: artistID, Depth: depth, CoArtists: out}
			if len(out) == 0 {
				result.Note = "no co-tour artists found"
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&artistID, "artist", "", "Artist ID (required)")
	cmd.Flags().IntVar(&depth, "depth", 1, "Graph hops to explore")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
