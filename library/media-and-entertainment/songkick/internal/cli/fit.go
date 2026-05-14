// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/songkick/internal/store"
)

type fitBrief struct {
	Anchor  string  `json:"anchor"`
	On      string  `json:"on"`
	Window  int     `json:"window"`
	Radius  float64 `json:"radius"`
	Weights struct {
		Routing     float64 `json:"routing"`
		Popularity  float64 `json:"popularity"`
		Reliability float64 `json:"reliability"`
	} `json:"weights"`
}

type fitEntry struct {
	ArtistID    string  `json:"artist_id"`
	DisplayName string  `json:"displayName,omitempty"`
	Composite   float64 `json:"composite"`
	Routing     float64 `json:"routing"`
	Popularity  float64 `json:"popularity"`
	Reliability float64 `json:"reliability"`
}

func newFitCmd(flags *rootFlags) *cobra.Command {
	var briefPath, shortlist, dbPath string
	cmd := &cobra.Command{
		Use:   "fit",
		Short: "Score shortlist against a brief.yml",
		Example: strings.Trim(`
  songkick-pp-cli fit --brief brief.yml --shortlist short.csv --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("brief") && !cmd.Flags().Changed("shortlist") {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if briefPath == "" {
				return apiErr(fmt.Errorf("brief file not found: --brief is required"))
			}
			brief, err := loadBrief(briefPath)
			if err != nil {
				return apiErr(fmt.Errorf("brief file not found: %w", err))
			}
			if shortlist == "" {
				return flags.printJSON(cmd, []fitEntry{})
			}
			rows, err := readShortlistCSV(shortlist)
			if err != nil {
				return flags.printJSON(cmd, []fitEntry{})
			}
			target, err := time.Parse("2006-01-02", brief.On)
			if err != nil {
				return usageErr(fmt.Errorf("brief.on must be YYYY-MM-DD: %w", err))
			}
			aLat, aLng, ok := parseAnchor(brief.Anchor)
			if !ok {
				return usageErr(fmt.Errorf("unknown brief.anchor %q", brief.Anchor))
			}
			if brief.Window == 0 {
				brief.Window = 14
			}
			if brief.Radius == 0 {
				brief.Radius = 5000
			}
			wRoute := brief.Weights.Routing
			wPop := brief.Weights.Popularity
			wRel := brief.Weights.Reliability
			if wRoute == 0 && wPop == 0 && wRel == 0 {
				wRoute, wPop, wRel = 0.5, 0.3, 0.2
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

			out := make([]fitEntry, 0, len(rows))
			for _, r := range rows {
				cands := scoreCandidatesForArtist(events, r.id, target, aLat, aLng, brief.Window, brief.Radius)
				route := 0.0
				if len(cands) > 0 {
					best := cands[0]
					for _, c := range cands[1:] {
						if c.Score > best.Score {
							best = c
						}
					}
					route = best.Score
				}
				wantID := asInt64(r.id)
				var popSum float64
				var popN int
				cancelled := 0
				total := 0
				for _, ev := range events {
					if !artistMatches(ev, wantID, r.id) {
						continue
					}
					if ev.Popularity > 0 {
						popSum += ev.Popularity
						popN++
					}
					total++
					if s := strings.ToLower(ev.Status); s == "cancelled" || s == "canceled" {
						cancelled++
					}
				}
				pop := 0.0
				if popN > 0 {
					pop = popSum / float64(popN)
				}
				rel := 1.0
				if total > 0 {
					rel = 1.0 - float64(cancelled)/float64(total)
				}
				composite := wRoute*route + wPop*pop*100 + wRel*rel*100
				out = append(out, fitEntry{
					ArtistID:    r.id,
					DisplayName: r.name,
					Composite:   math.Round(composite*10) / 10,
					Routing:     math.Round(route*10) / 10,
					Popularity:  math.Round(pop*10000) / 10000,
					Reliability: math.Round(rel*10000) / 10000,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Composite > out[j].Composite })
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&briefPath, "brief", "", "Brief file path (YAML or JSON)")
	cmd.Flags().StringVar(&shortlist, "shortlist", "", "CSV shortlist path")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func loadBrief(path string) (fitBrief, error) {
	var b fitBrief
	f, err := os.Open(path)
	if err != nil {
		return b, err
	}
	defer f.Close()
	// Read full content
	var sb strings.Builder
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
		sb.WriteByte('\n')
	}
	content := strings.TrimSpace(sb.String())
	if strings.HasPrefix(content, "{") {
		if err := json.Unmarshal([]byte(content), &b); err != nil {
			return b, err
		}
		return b, nil
	}
	// Tolerant YAML parser for our flat schema with one nested "weights:" block.
	inWeights := false
	for _, line := range strings.Split(content, "\n") {
		raw := line
		stripped := strings.TrimLeft(raw, " \t")
		if stripped == "" || strings.HasPrefix(stripped, "#") {
			continue
		}
		indent := len(raw) - len(stripped)
		key, val, found := strings.Cut(stripped, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if indent == 0 {
			inWeights = false
		}
		if key == "weights" && val == "" {
			inWeights = true
			continue
		}
		if inWeights && indent > 0 {
			f, _ := strconv.ParseFloat(val, 64)
			switch key {
			case "routing":
				b.Weights.Routing = f
			case "popularity":
				b.Weights.Popularity = f
			case "reliability":
				b.Weights.Reliability = f
			}
			continue
		}
		switch key {
		case "anchor":
			b.Anchor = val
		case "on":
			b.On = val
		case "window":
			n, _ := strconv.Atoi(val)
			b.Window = n
		case "radius":
			f, _ := strconv.ParseFloat(val, 64)
			b.Radius = f
		}
	}
	return b, nil
}
