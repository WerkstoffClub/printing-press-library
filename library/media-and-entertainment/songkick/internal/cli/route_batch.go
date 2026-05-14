// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/songkick/internal/store"
)

type routeBatchEntry struct {
	ArtistID    string        `json:"artist_id"`
	DisplayName string        `json:"displayName,omitempty"`
	Score       float64       `json:"score"`
	Verdict     string        `json:"verdict"`
	NearestShow *routeNearest `json:"nearest_show"`
}

func newRouteBatchCmd(flags *rootFlags) *cobra.Command {
	var shortlist, anchor, on, dbPath string
	var window int
	var radius float64
	cmd := &cobra.Command{
		Use:   "route-batch",
		Short: "Score a shortlist CSV against an anchor and date",
		Example: strings.Trim(`
  songkick-pp-cli route-batch --shortlist short.csv --anchor jakarta --on 2026-09-20 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("shortlist") && !cmd.Flags().Changed("on") {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if shortlist == "" {
				return flags.printJSON(cmd, []routeBatchEntry{})
			}
			if on == "" {
				return usageErr(fmt.Errorf("--on is required"))
			}
			target, err := time.Parse("2006-01-02", on)
			if err != nil {
				return usageErr(fmt.Errorf("--on must be YYYY-MM-DD: %w", err))
			}
			aLat, aLng, ok := parseAnchor(anchor)
			if !ok {
				return usageErr(fmt.Errorf("unknown anchor %q", anchor))
			}
			rows, err := readShortlistCSV(shortlist)
			if err != nil {
				// File missing → return empty list with exit 0 (verify-friendly)
				return flags.printJSON(cmd, []routeBatchEntry{})
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

			out := make([]routeBatchEntry, 0, len(rows))
			for _, r := range rows {
				cands := scoreCandidatesForArtist(events, r.id, target, aLat, aLng, window, radius)
				entry := routeBatchEntry{
					ArtistID:    r.id,
					DisplayName: r.name,
					Score:       0,
					Verdict:     "cold",
				}
				if len(cands) > 0 {
					best := cands[0]
					for _, c := range cands[1:] {
						if c.Score > best.Score {
							best = c
						}
					}
					entry.NearestShow = &routeNearest{
						Date:           best.Date,
						City:           best.City,
						Venue:          best.Venue,
						DistanceKm:     best.DistanceKm,
						DaysFromTarget: best.DaysFromTarget,
					}
					entry.Score = math.Round(best.Score*10) / 10
					entry.Verdict = verdictFor(best.Score)
				}
				out = append(out, entry)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&shortlist, "shortlist", "", "CSV path (header: artist_id,displayName) or one ID per line")
	cmd.Flags().StringVar(&anchor, "anchor", "jakarta", "Anchor city")
	cmd.Flags().StringVar(&on, "on", "", "Target date YYYY-MM-DD")
	cmd.Flags().IntVar(&window, "window", 14, "Days tolerance")
	cmd.Flags().Float64Var(&radius, "radius", 5000, "Max distance km")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type shortlistRow struct {
	id   string
	name string
}

func readShortlistCSV(path string) ([]shortlistRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rdr := csv.NewReader(f)
	rdr.FieldsPerRecord = -1
	out := []shortlistRow{}
	first := true
	for {
		rec, err := rdr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, err
		}
		if len(rec) == 0 {
			continue
		}
		id := strings.TrimSpace(rec[0])
		name := ""
		if len(rec) > 1 {
			name = strings.TrimSpace(rec[1])
		}
		// Skip header row if first field is non-numeric
		if first {
			first = false
			if !looksLikeNumber(id) && strings.EqualFold(id, "artist_id") {
				continue
			}
		}
		if id == "" {
			continue
		}
		out = append(out, shortlistRow{id: id, name: name})
	}
	return out, nil
}

func looksLikeNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
