// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// `report consolidate` — merge N open drafts into one target draft, leaving
// the source drafts untouched (empty but harmless). See
// docs/plans/2026-04-20-006-feat-expensify-live-data-and-consolidate-plan.md
// for the approach.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/store"

	"github.com/spf13/cobra"
)

// ReportMutator isolates the two write endpoints that `report consolidate`
// needs so tests can supply a spy without wiring a real HTTP client.
type ReportMutator interface {
	AddExpenses(reportID string, txIDs []string) error
	SubmitReport(reportID string) error
}

// httpReportMutator calls the real Expensify dispatcher via the existing
// client. Body shapes mirror `report_add.go` and `report_submit.go` verbatim
// per the plan's write-path freeze.
type httpReportMutator struct {
	post func(path string, body any) (json.RawMessage, int, error)
}

func (m *httpReportMutator) AddExpenses(reportID string, txIDs []string) error {
	body := map[string]any{
		"reportID":       reportID,
		"transactionIDs": strings.Join(txIDs, ","),
	}
	_, status, err := m.post("/AddExpensesToReport", body)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("AddExpensesToReport returned HTTP %d", status)
	}
	return nil
}

func (m *httpReportMutator) SubmitReport(reportID string) error {
	body := map[string]any{"reportID": reportID}
	_, status, err := m.post("/SubmitReport", body)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("SubmitReport returned HTTP %d", status)
	}
	return nil
}

// consolidateOptions collects the CLI flags into a struct so consolidateOp
// (the test-friendly core) has a stable signature.
type consolidateOptions struct {
	targetID string
	since    string
	until    string
	title    string
	policyID string
	submit   bool
	dryRun   bool
}

// consolidateOp runs the filter → target-pick → expense-gather → preview →
// (optional) AddExpenses → (optional) SubmitReport pipeline. It returns an
// error following the usageErr / apiErr convention used elsewhere.
//
// The function is intentionally small and deterministic so tests can drive
// it with a fake store and a spy mutator.
func consolidateOp(w io.Writer, st *store.Store, cfg *config.Config, mut ReportMutator, opts consolidateOptions) error {
	// 1. Gather candidate drafts. `status = "open"` is the string form; we
	//    ALSO filter on stateNum=0 below when present so we honor the real
	//    lifecycle signal even on rows where status is blank.
	filters := map[string]string{}
	if opts.policyID != "" {
		filters["policy_id"] = opts.policyID
	}
	reports, err := st.ListReports(filters)
	if err != nil {
		return apiErr(fmt.Errorf("ListReports: %w", err))
	}

	var since, until time.Time
	if opts.since != "" {
		since, err = time.Parse("2006-01-02", opts.since)
		if err != nil {
			return usageErr(fmt.Errorf("--since must be YYYY-MM-DD: %w", err))
		}
	}
	if opts.until != "" {
		until, err = time.Parse("2006-01-02", opts.until)
		if err != nil {
			return usageErr(fmt.Errorf("--until must be YYYY-MM-DD: %w", err))
		}
	}

	var candidates []store.Report
	for _, r := range reports {
		// Open-draft test: stateNum==0 when present wins; else lowercased
		// status must be "open". Rows with neither are excluded.
		if !isOpenDraft(r) {
			continue
		}
		if !since.IsZero() {
			if t, ok := parseReportCreated(r.Created); ok {
				if t.Before(since) {
					continue
				}
			}
		}
		if !until.IsZero() {
			if t, ok := parseReportCreated(r.Created); ok {
				if t.After(until) {
					continue
				}
			}
		}
		if cfg != nil && cfg.ExpensifyAccountID != 0 {
			ownerID := ownerAccountIDFromRaw(r.RawJSON)
			if ownerID != 0 && ownerID != cfg.ExpensifyAccountID {
				continue
			}
		}
		candidates = append(candidates, r)
	}

	// 2. Guards.
	if len(candidates) == 0 {
		fmt.Fprintln(w, "No open drafts found. Nothing to consolidate.")
		return nil
	}
	if len(candidates) == 1 {
		fmt.Fprintln(w, "Only 1 draft found. Nothing to consolidate.")
		return nil
	}
	if opts.policyID == "" {
		policySet := map[string]struct{}{}
		for _, c := range candidates {
			policySet[c.PolicyID] = struct{}{}
		}
		if len(policySet) > 1 {
			var policies []string
			for p := range policySet {
				policies = append(policies, p)
			}
			sort.Strings(policies)
			return usageErr(fmt.Errorf("Cannot merge drafts across different workspaces: [%s]. Use --policy-id <id> to scope.", strings.Join(policies, ", ")))
		}
	}

	// 3. Pick the target.
	var target store.Report
	if opts.targetID != "" {
		var found *store.Report
		for i := range candidates {
			if candidates[i].ReportID == opts.targetID {
				found = &candidates[i]
				break
			}
		}
		// Fallback: allow --target to name any open-draft in the store even
		// when it wasn't in the date-filtered candidate set, as long as it
		// IS an open draft (stateNum=0) and exists.
		if found == nil {
			for _, r := range reports {
				if r.ReportID == opts.targetID {
					rr := r
					if !isOpenDraft(rr) {
						return usageErr(fmt.Errorf("--target %s is not an open draft (stateNum=%d status=%q)", opts.targetID, rr.StateNum, rr.Status))
					}
					found = &rr
					break
				}
			}
		}
		if found == nil {
			return usageErr(fmt.Errorf("--target %s not found in local store (run 'expensify-pp-cli sync' or check the id)", opts.targetID))
		}
		target = *found
	} else {
		// Oldest-by-created wins.
		sorted := make([]store.Report, len(candidates))
		copy(sorted, candidates)
		sort.SliceStable(sorted, func(i, j int) bool {
			return sorted[i].Created < sorted[j].Created
		})
		target = sorted[0]
	}

	// 4. Collect expenses from source drafts.
	var sources []store.Report
	for _, c := range candidates {
		if c.ReportID == target.ReportID {
			continue
		}
		sources = append(sources, c)
	}

	type perSource struct {
		report   store.Report
		expenses []store.Expense
	}
	var perSources []perSource
	var allTxIDs []string
	var totalToMerge int64
	for _, s := range sources {
		exps, err := st.ListExpenses(map[string]string{"report_id": s.ReportID})
		if err != nil {
			return apiErr(fmt.Errorf("ListExpenses for %s: %w", s.ReportID, err))
		}
		perSources = append(perSources, perSource{report: s, expenses: exps})
		for _, e := range exps {
			allTxIDs = append(allTxIDs, e.TransactionID)
			totalToMerge += e.Amount
		}
	}

	// 5. Dry-run preview.
	fmt.Fprintf(w, "Will consolidate %d drafts totaling $%.2f into report %s (%s):\n",
		len(sources), float64(totalToMerge)/100, target.ReportID, target.Title)
	for _, ps := range perSources {
		var srcTotal int64
		for _, e := range ps.expenses {
			srcTotal += e.Amount
		}
		fmt.Fprintf(w, "  - %s ($%.2f, %d expenses) -> to merge\n",
			ps.report.Title, float64(srcTotal)/100, len(ps.expenses))
	}
	fmt.Fprintf(w, "Attaching %d expenses total. Source drafts will remain empty (delete with report delete <id> if desired).\n", len(allTxIDs))
	if opts.title != "" && opts.targetID == "" {
		fmt.Fprintf(w, "Note: --title %q not applied — this unit reuses the oldest draft as target; rename it after the fact if needed.\n", opts.title)
	}

	if opts.dryRun {
		return nil
	}

	// 6. Execute. Skip AddExpenses if there is nothing to merge.
	attached := 0
	if len(allTxIDs) > 0 {
		if err := mut.AddExpenses(target.ReportID, allTxIDs); err != nil {
			return apiErr(fmt.Errorf("Attached %d of %d expenses. Failed: %v. Target id=%s has the attached expenses.",
				attached, len(allTxIDs), err, target.ReportID))
		}
		attached = len(allTxIDs)
	}

	// 7. Optional submit.
	if opts.submit {
		if err := mut.SubmitReport(target.ReportID); err != nil {
			return apiErr(fmt.Errorf("SubmitReport(%s) after attaching %d expenses: %w", target.ReportID, attached, err))
		}
		fmt.Fprintf(w, "Submitted report %s.\n", target.ReportID)
	}
	fmt.Fprintf(w, "Consolidated %d expenses into report %s.\n", attached, target.ReportID)
	return nil
}

// isOpenDraft returns true when the report is still in the OPEN (stateNum=0)
// lifecycle state. When stateNum is unset (legacy row), we fall back to the
// lowercased status string.
func isOpenDraft(r store.Report) bool {
	if r.StateNum != 0 {
		return false
	}
	// stateNum==0 from a non-default insert is the happy path. Rows that
	// never had stateNum populated also land here; for those we need a
	// secondary signal.
	if r.Status == "" {
		return true
	}
	lower := strings.ToLower(r.Status)
	return lower == "open" || lower == "draft"
}

// parseReportCreated accepts either "YYYY-MM-DD" or an RFC3339-ish timestamp
// and returns the first 10 chars parsed as a date. Returns (zero, false) on
// parse failure so callers can skip the filter rather than erroring out.
func parseReportCreated(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05Z", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// ownerAccountIDFromRaw looks for ownerAccountID (or accountID) inside the
// report's archived raw_json. Returns 0 when the field is absent or the JSON
// is malformed; callers treat that as "unknown" and keep the row.
func ownerAccountIDFromRaw(raw string) int64 {
	if raw == "" {
		return 0
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return 0
	}
	for _, k := range []string{"ownerAccountID", "ownerAccountId", "accountID", "accountId"} {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int64:
			return n
		case int:
			return int64(n)
		case string:
			var i int64
			if _, err := fmt.Sscanf(n, "%d", &i); err == nil {
				return i
			}
		}
	}
	return 0
}

func newReportConsolidateCmd(flags *rootFlags) *cobra.Command {
	var opts consolidateOptions
	cmd := &cobra.Command{
		Use:   "consolidate",
		Short: "Merge N open drafts into one target report",
		Long: `Merge expenses from N open draft reports into a single target draft.

By default the oldest draft becomes the target; pass --target to pick a
specific report. Source drafts are left untouched (they become empty but
remain in the store); run 'report delete <id>' afterward to tidy up.`,
		Example: `  expensify-pp-cli report consolidate --since 2026-04-01 --policy-id ABC
  expensify-pp-cli report consolidate --target REPORT_X --submit
  expensify-pp-cli report consolidate --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := store.Open("")
			if err != nil {
				return configErr(err)
			}
			defer st.Close()

			cfg, cfgErr := config.Load(flags.configPath)
			if cfgErr != nil {
				// Config load failure is non-fatal for read paths; the
				// consolidate pipeline only uses ExpensifyAccountID for an
				// optional filter. Proceed with a zero-value config so the
				// filter is simply skipped.
				cfg = &config.Config{}
			}

			// --dry-run piggybacks on the root flag too.
			if flags.dryRun {
				opts.dryRun = true
			}

			var mut ReportMutator
			if !opts.dryRun {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				mut = &httpReportMutator{post: c.Post}
			}
			return consolidateOp(cmd.OutOrStdout(), st, cfg, mut, opts)
		},
	}
	cmd.Flags().StringVar(&opts.targetID, "target", "", "Reuse this report as the destination (must be an existing open draft)")
	cmd.Flags().StringVar(&opts.since, "since", "", "Only consider drafts created on or after YYYY-MM-DD")
	cmd.Flags().StringVar(&opts.until, "until", "", "Only consider drafts created on or before YYYY-MM-DD")
	cmd.Flags().StringVar(&opts.title, "title", "", "Desired target title (note: this unit reuses the oldest draft; --title is surfaced in the preview only)")
	cmd.Flags().StringVar(&opts.policyID, "policy-id", "", "Restrict candidates to this policy/workspace")
	cmd.Flags().BoolVar(&opts.submit, "submit", false, "Submit the target report after attaching expenses")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Preview the plan without calling the API")
	return cmd
}
