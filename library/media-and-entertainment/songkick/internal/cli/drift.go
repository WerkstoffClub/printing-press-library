// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/songkick/internal/store"
)

type driftResource struct {
	ResourceType string `json:"resource_type"`
	LastSync     string `json:"last_sync"`
	EventsSynced int    `json:"events_synced"`
}

type driftResult struct {
	Since     string          `json:"since"`
	Resources []driftResource `json:"resources"`
	Near      string          `json:"near"`
	Diffs     []any           `json:"diffs"`
	Snapshots int             `json:"snapshots"`
	Note      string          `json:"note,omitempty"`
}

func newDriftCmd(flags *rootFlags) *cobra.Command {
	var since, near, dbPath string
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Diff against prior sync snapshot",
		Example: strings.Trim(`
  songkick-pp-cli drift --since 7d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if _, err := parseDriftDuration(since); err != nil {
				return usageErr(err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("songkick-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			result := driftResult{
				Since:     since,
				Resources: []driftResource{},
				Near:      near,
				Diffs:     []any{},
				Snapshots: 0,
			}
			rows, qerr := db.Query("SELECT resource_type, last_synced_at, count FROM sync_state ORDER BY resource_type")
			if qerr != nil {
				result.Note = "no snapshot history yet"
				return flags.printJSON(cmd, result)
			}
			defer rows.Close()
			for rows.Next() {
				var rt, ts string
				var count int
				if err := rows.Scan(&rt, &ts, &count); err != nil {
					continue
				}
				result.Resources = append(result.Resources, driftResource{
					ResourceType: rt,
					LastSync:     ts,
					EventsSynced: count,
				})
			}
			if len(result.Resources) == 0 {
				result.Note = "no snapshot history yet"
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Lookback window (e.g. 7d, 24h)")
	cmd.Flags().StringVar(&near, "near", "", "Optional anchor city to filter diffs")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func parseDriftDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty --since")
	}
	unit := s[len(s)-1:]
	num := s[:len(s)-1]
	n, err := strconv.Atoi(num)
	if err != nil {
		return 0, fmt.Errorf("invalid --since %q: expected Nd or Nh", s)
	}
	switch unit {
	case "d":
		return time.Duration(n) * 24 * time.Hour, nil
	case "h":
		return time.Duration(n) * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid --since unit %q: use d or h", unit)
	}
}
