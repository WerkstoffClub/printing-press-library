// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
//
// Tests for the `report consolidate` pipeline. We drive the helper
// consolidateOp directly with a real SQLite store and a spyMutator so no
// HTTP client is needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/store"
)

// spyMutator records the calls consolidateOp makes instead of hitting the
// network. Tests inspect the recorded calls to assert ordering and shape.
type spyMutator struct {
	addCalls    []spyAddCall
	submitCalls []string
	addErr      error
	submitErr   error
	order       []string // tracks "add" vs "submit" to verify ordering
}

type spyAddCall struct {
	reportID string
	txIDs    []string
}

func (s *spyMutator) AddExpenses(reportID string, txIDs []string) error {
	cp := make([]string, len(txIDs))
	copy(cp, txIDs)
	s.addCalls = append(s.addCalls, spyAddCall{reportID: reportID, txIDs: cp})
	s.order = append(s.order, "add")
	return s.addErr
}

func (s *spyMutator) SubmitReport(reportID string) error {
	s.submitCalls = append(s.submitCalls, reportID)
	s.order = append(s.order, "submit")
	return s.submitErr
}

// seedDraft upserts an OPEN draft report (stateNum=0) with the given fields.
func seedDraft(t *testing.T, st *store.Store, id, policy, title, created string, total int64) {
	t.Helper()
	r := store.Report{
		ReportID:    id,
		PolicyID:    policy,
		Title:       title,
		Status:      "open",
		Total:       total,
		Currency:    "USD",
		Created:     created,
		LastUpdated: created,
		StateNum:    0,
	}
	if err := st.UpsertReport(r); err != nil {
		t.Fatalf("UpsertReport(%s): %v", id, err)
	}
}

// seedExpenseOnReport inserts a single expense attached to reportID.
func seedExpenseOnReport(t *testing.T, st *store.Store, txID, reportID, policy string, amount int64) {
	t.Helper()
	e := store.Expense{
		TransactionID: txID,
		ReportID:      reportID,
		Merchant:      "Test Merchant " + txID,
		Amount:        amount,
		Currency:      "USD",
		Date:          "2026-04-10",
		PolicyID:      policy,
	}
	if err := st.UpsertExpense(e); err != nil {
		t.Fatalf("UpsertExpense(%s): %v", txID, err)
	}
}

// TestConsolidate_NoDrafts: empty store → success with explicit message.
func TestConsolidate_NoDrafts(t *testing.T) {
	st := openTestStore(t)
	cfg := &config.Config{}
	spy := &spyMutator{}
	var out bytes.Buffer

	err := consolidateOp(&out, st, cfg, spy, consolidateOptions{})
	if err != nil {
		t.Fatalf("consolidateOp: %v", err)
	}
	if !strings.Contains(out.String(), "No open drafts found") {
		t.Fatalf("stdout = %q, want 'No open drafts found'", out.String())
	}
	if len(spy.addCalls) != 0 || len(spy.submitCalls) != 0 {
		t.Fatalf("spy calls = %+v, want zero", spy)
	}
}

// TestConsolidate_OneDraft: single open draft → success with explicit message.
func TestConsolidate_OneDraft(t *testing.T) {
	st := openTestStore(t)
	seedDraft(t, st, "r1", "POL", "Only draft", "2026-04-01", 1000)
	cfg := &config.Config{}
	spy := &spyMutator{}
	var out bytes.Buffer

	err := consolidateOp(&out, st, cfg, spy, consolidateOptions{})
	if err != nil {
		t.Fatalf("consolidateOp: %v", err)
	}
	if !strings.Contains(out.String(), "Only 1 draft found") {
		t.Fatalf("stdout = %q, want 'Only 1 draft found'", out.String())
	}
	if len(spy.addCalls) != 0 || len(spy.submitCalls) != 0 {
		t.Fatalf("spy calls = %+v, want zero", spy)
	}
}

// TestConsolidate_CrossWorkspaceRefused: two drafts in two policies with no
// --policy-id → usageErr (exit 2) with both policy IDs in the message.
func TestConsolidate_CrossWorkspaceRefused(t *testing.T) {
	st := openTestStore(t)
	seedDraft(t, st, "r1", "POL_A", "Draft A", "2026-04-01", 1000)
	seedDraft(t, st, "r2", "POL_B", "Draft B", "2026-04-02", 2000)
	cfg := &config.Config{}
	spy := &spyMutator{}
	var out bytes.Buffer

	err := consolidateOp(&out, st, cfg, spy, consolidateOptions{})
	if err == nil {
		t.Fatalf("expected error, got nil (out=%q)", out.String())
	}
	if ExitCode(err) != 2 {
		t.Fatalf("exit code = %d, want 2 (err=%v)", ExitCode(err), err)
	}
	msg := err.Error()
	for _, want := range []string{"POL_A", "POL_B", "--policy-id"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q missing %q", msg, want)
		}
	}
	if len(spy.addCalls) != 0 || len(spy.submitCalls) != 0 {
		t.Fatalf("spy calls = %+v, want zero", spy)
	}
}

// TestConsolidate_DryRun_NoCalls: 3 drafts same policy + --dry-run → preview
// on stdout, zero API calls.
func TestConsolidate_DryRun_NoCalls(t *testing.T) {
	st := openTestStore(t)
	seedDraft(t, st, "r1", "POL", "Draft 1", "2026-04-01", 1000)
	seedDraft(t, st, "r2", "POL", "Draft 2", "2026-04-02", 2000)
	seedDraft(t, st, "r3", "POL", "Draft 3", "2026-04-03", 3000)
	seedExpenseOnReport(t, st, "t2", "r2", "POL", 2000)
	seedExpenseOnReport(t, st, "t3", "r3", "POL", 3000)
	cfg := &config.Config{}
	spy := &spyMutator{}
	var out bytes.Buffer

	err := consolidateOp(&out, st, cfg, spy, consolidateOptions{dryRun: true})
	if err != nil {
		t.Fatalf("consolidateOp: %v", err)
	}
	if len(spy.addCalls) != 0 || len(spy.submitCalls) != 0 {
		t.Fatalf("spy calls = %+v, want zero on dry-run", spy)
	}
	s := out.String()
	for _, want := range []string{"Will consolidate", "Draft 1", "Draft 2", "Draft 3", "Attaching 2 expenses"} {
		if !strings.Contains(s, want) {
			t.Fatalf("stdout missing %q\nfull output:\n%s", want, s)
		}
	}
}

// TestConsolidate_TargetExplicit_NotDraft: --target points to a closed
// report → usageErr mentioning the state.
func TestConsolidate_TargetExplicit_NotDraft(t *testing.T) {
	st := openTestStore(t)
	// Two open drafts (needed to clear the "< 2 candidates" guard).
	seedDraft(t, st, "r1", "POL", "Draft 1", "2026-04-01", 1000)
	seedDraft(t, st, "r2", "POL", "Draft 2", "2026-04-02", 2000)
	// Target is a submitted report (stateNum=3 Approved).
	target := store.Report{
		ReportID:    "r-closed",
		PolicyID:    "POL",
		Title:       "Closed report",
		Status:      "approved",
		Total:       5000,
		Currency:    "USD",
		Created:     "2026-03-01",
		LastUpdated: "2026-03-05",
		StateNum:    3,
	}
	if err := st.UpsertReport(target); err != nil {
		t.Fatalf("UpsertReport: %v", err)
	}

	cfg := &config.Config{}
	spy := &spyMutator{}
	var out bytes.Buffer

	err := consolidateOp(&out, st, cfg, spy, consolidateOptions{targetID: "r-closed"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if ExitCode(err) != 2 {
		t.Fatalf("exit code = %d, want 2 (err=%v)", ExitCode(err), err)
	}
	if !strings.Contains(err.Error(), "not an open draft") {
		t.Fatalf("error %q missing 'not an open draft'", err.Error())
	}
}

// TestConsolidate_TargetExplicit_NotFound: nonexistent reportID →
// usageErr (exit 2) with the id in the message.
func TestConsolidate_TargetExplicit_NotFound(t *testing.T) {
	st := openTestStore(t)
	seedDraft(t, st, "r1", "POL", "Draft 1", "2026-04-01", 1000)
	seedDraft(t, st, "r2", "POL", "Draft 2", "2026-04-02", 2000)
	cfg := &config.Config{}
	spy := &spyMutator{}
	var out bytes.Buffer

	err := consolidateOp(&out, st, cfg, spy, consolidateOptions{targetID: "does-not-exist"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if ExitCode(err) != 2 {
		t.Fatalf("exit code = %d, want 2 (err=%v)", ExitCode(err), err)
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Fatalf("error %q missing 'does-not-exist'", err.Error())
	}
}

// TestConsolidate_HappyPath_NoSubmit: 3 drafts, consolidate without --submit.
// Oldest draft (r1) is the target; expenses from r2 + r3 are collected and
// passed to AddExpensesToReport as one call. SubmitReport is never called.
func TestConsolidate_HappyPath_NoSubmit(t *testing.T) {
	st := openTestStore(t)
	seedDraft(t, st, "r1", "POL", "Oldest", "2026-04-01", 0)
	seedDraft(t, st, "r2", "POL", "Middle", "2026-04-02", 2000)
	seedDraft(t, st, "r3", "POL", "Youngest", "2026-04-03", 3000)
	seedExpenseOnReport(t, st, "t2a", "r2", "POL", 500)
	seedExpenseOnReport(t, st, "t2b", "r2", "POL", 1500)
	seedExpenseOnReport(t, st, "t3", "r3", "POL", 3000)

	cfg := &config.Config{}
	spy := &spyMutator{}
	var out bytes.Buffer

	err := consolidateOp(&out, st, cfg, spy, consolidateOptions{})
	if err != nil {
		t.Fatalf("consolidateOp: %v\noutput:\n%s", err, out.String())
	}
	if len(spy.addCalls) != 1 {
		t.Fatalf("addCalls = %d, want 1; full spy: %+v", len(spy.addCalls), spy)
	}
	if spy.addCalls[0].reportID != "r1" {
		t.Fatalf("AddExpenses target = %s, want r1", spy.addCalls[0].reportID)
	}
	gotIDs := map[string]bool{}
	for _, id := range spy.addCalls[0].txIDs {
		gotIDs[id] = true
	}
	for _, want := range []string{"t2a", "t2b", "t3"} {
		if !gotIDs[want] {
			t.Fatalf("AddExpenses txIDs missing %q; got %v", want, spy.addCalls[0].txIDs)
		}
	}
	if len(spy.submitCalls) != 0 {
		t.Fatalf("submitCalls = %v, want none", spy.submitCalls)
	}
}

// TestConsolidate_HappyPath_WithSubmit: same as above but with --submit. Both
// endpoints are called; add must come first.
func TestConsolidate_HappyPath_WithSubmit(t *testing.T) {
	st := openTestStore(t)
	seedDraft(t, st, "r1", "POL", "Oldest", "2026-04-01", 0)
	seedDraft(t, st, "r2", "POL", "Middle", "2026-04-02", 2000)
	seedDraft(t, st, "r3", "POL", "Youngest", "2026-04-03", 3000)
	seedExpenseOnReport(t, st, "t2", "r2", "POL", 2000)
	seedExpenseOnReport(t, st, "t3", "r3", "POL", 3000)

	cfg := &config.Config{}
	spy := &spyMutator{}
	var out bytes.Buffer

	err := consolidateOp(&out, st, cfg, spy, consolidateOptions{submit: true})
	if err != nil {
		t.Fatalf("consolidateOp: %v\noutput:\n%s", err, out.String())
	}
	if len(spy.addCalls) != 1 {
		t.Fatalf("addCalls = %d, want 1; full spy: %+v", len(spy.addCalls), spy)
	}
	if len(spy.submitCalls) != 1 {
		t.Fatalf("submitCalls = %d, want 1; full spy: %+v", len(spy.submitCalls), spy)
	}
	if spy.submitCalls[0] != "r1" {
		t.Fatalf("SubmitReport target = %q, want r1", spy.submitCalls[0])
	}
	if len(spy.order) != 2 || spy.order[0] != "add" || spy.order[1] != "submit" {
		t.Fatalf("call order = %v, want [add, submit]", spy.order)
	}
}
