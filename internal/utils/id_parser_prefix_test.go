package utils

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/steveyegge/beads/internal/storage"
	"github.com/steveyegge/beads/internal/types"
)

// Regression coverage for [co-bfwpn]: partial issue IDs must resolve as a
// PREFIX of the hash, never as an arbitrary substring.
//
// Before the fix, ResolvePartialID filtered candidates with strings.Contains.
// With "co-fi9bx" in the store, `bd show co-9bx` and `bd show co-i9b` both
// silently resolved to it. Every ID-taking verb — show, close, update, delete,
// dep, assign — funnels through ResolvePartialID, so on a mutating verb that
// acted on a bead the operator never named.
//
// These tests use a stub store rather than the Dolt-backed newTestStore in
// id_parser_test.go, which skips whenever Docker is unavailable. The stub
// reproduces the two storage behaviours ResolvePartialID actually depends on:
// exact-ID lookup via SearchIssues(filter.IDs), and the SQL candidate-narrowing
// `id LIKE '%query%'` via SearchIssueIDs. Keeping the stub's narrowing as a
// substring is deliberate — it proves the prefix rule is enforced by the Go
// filter even when SQL hands back a wider superset.

// fakeIDStore implements just enough of storage.Storage for ResolvePartialID.
// The embedded nil interface panics on any method these tests don't exercise,
// which is the intended signal that the resolver's dependencies changed.
type fakeIDStore struct {
	storage.Storage

	prefix string
	ids    []string // non-ephemeral issue IDs
	wisps  []string // ephemeral (wisp) IDs
}

func (f *fakeIDStore) GetConfig(_ context.Context, key string) (string, error) {
	if key == "issue_prefix" {
		return f.prefix, nil
	}
	return "", nil
}

// SearchIssues serves only the exact-ID fast paths (filter.IDs set).
func (f *fakeIDStore) SearchIssues(_ context.Context, _ string, filter types.IssueFilter) ([]*types.Issue, error) {
	var out []*types.Issue
	for _, want := range filter.IDs {
		for _, id := range append(append([]string{}, f.ids...), f.wisps...) {
			if id == want {
				out = append(out, &types.Issue{ID: id})
			}
		}
	}
	return out, nil
}

// SearchIssueIDs mimics the production SQL narrowing step: `id LIKE '%query%'`.
// It intentionally returns non-prefix candidates so the caller's filter is what
// is under test.
func (f *fakeIDStore) SearchIssueIDs(_ context.Context, query string, filter types.IssueFilter) ([]string, error) {
	pool := f.ids
	if filter.Ephemeral != nil && *filter.Ephemeral {
		pool = f.wisps
	}
	var out []string
	for _, id := range pool {
		if strings.Contains(id, query) {
			out = append(out, id)
		}
	}
	return out, nil
}

func newFakeIDStore() *fakeIDStore {
	return &fakeIDStore{
		prefix: "co",
		ids: []string{
			"co-fi9bx",
			"co-9b4",
			"co-59b4",
			"co-bfwpn",
			"co-3q1cu",
			// Hierarchical child. Carries the prefix-vs-substring cases that
			// TestHashMatchesPartial covered directly; that unit test went away
			// with the hashMatchesPartial helper on the 2026-08-09 upstream merge
			// [co-gmlf3], so its coverage moves here, end-to-end.
			"co-3d0.1",
		},
	}
}

func TestResolvePartialID_PrefixNotSubstring(t *testing.T) {
	ctx := context.Background()
	store := newFakeIDStore()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string // substring the error must contain; "" means success
	}{
		// --- exact IDs keep working ---
		{
			name:  "exact id with prefix",
			input: "co-fi9bx",
			want:  "co-fi9bx",
		},
		{
			name:  "exact hash without prefix",
			input: "fi9bx",
			want:  "co-fi9bx",
		},

		// --- genuine prefixes still resolve ---
		{
			name:  "unique prefix resolves",
			input: "co-fi",
			want:  "co-fi9bx",
		},
		{
			name:  "unique prefix without id prefix resolves",
			input: "bfw",
			want:  "co-bfwpn",
		},

		// --- the co-bfwpn regression: non-prefix fragments must NOT resolve ---
		{
			name:    "trailing fragment does not resolve",
			input:   "co-9bx",
			wantErr: "no issue found matching",
		},
		{
			name:    "middle fragment does not resolve",
			input:   "co-i9b",
			wantErr: "no issue found matching",
		},
		{
			name:    "bare trailing fragment does not resolve",
			input:   "9bx",
			wantErr: "no issue found matching",
		},

		// --- hierarchical children follow the same prefix rule ---
		{
			name:  "hierarchical child by parent prefix resolves",
			input: "co-3d0",
			want:  "co-3d0.1",
		},
		{
			name:    "hierarchical non-prefix fragment does not resolve",
			input:   "co-d0.1",
			wantErr: "no issue found matching",
		},

		// --- unrelated IDs still error cleanly ---
		{
			name:    "nonexistent id errors",
			input:   "co-zzzzz",
			wantErr: "no issue found matching",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolvePartialID(ctx, store, tt.input)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ResolvePartialID(%q) = %q, want error containing %q", tt.input, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ResolvePartialID(%q) error = %q, want error containing %q", tt.input, err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ResolvePartialID(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ResolvePartialID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// An ambiguous prefix must refuse to guess and must name every candidate.
func TestResolvePartialID_AmbiguousPrefixListsCandidates(t *testing.T) {
	ctx := context.Background()
	store := newFakeIDStore()

	// "9b" prefixes co-9b4 only; add a second 9b* bead to force ambiguity.
	store.ids = append(store.ids, "co-9bqqq")

	_, err := ResolvePartialID(ctx, store, "co-9b")
	if err == nil {
		t.Fatal("ResolvePartialID(\"co-9b\") succeeded; want ambiguity error")
	}
	msg := err.Error()

	// Assert on the sentinel rather than the wording. This used to match the
	// literal "ambiguous ID"; upstream introduced ErrAmbiguousID ("ambiguous
	// issue ID") and the string assertion broke on the 2026-08-09 merge without
	// any behaviour changing [co-gmlf3]. errors.Is cannot rot the same way.
	if !errors.Is(err, ErrAmbiguousID) {
		t.Errorf("error %q is not ErrAmbiguousID", msg)
	}
	for _, want := range []string{"co-9b4", "co-9bqqq"} {
		if !strings.Contains(msg, want) {
			t.Errorf("ambiguity error %q does not list candidate %q", msg, want)
		}
	}
	// co-59b4 contains "9b" but is not prefixed by it. Under the old substring
	// rule it was reported as a third candidate; it must not appear now.
	if strings.Contains(msg, "co-59b4") {
		t.Errorf("ambiguity error %q lists co-59b4, which is a substring match, not a prefix match", msg)
	}
}

// An exact ID must win even when other beads share it as a prefix — the gh-316
// parent/hierarchical-child case, re-asserted so the prefix change cannot
// regress it.
func TestResolvePartialID_ExactBeatsLongerPrefixMatch(t *testing.T) {
	ctx := context.Background()
	store := &fakeIDStore{
		prefix: "co",
		ids:    []string{"co-3d0", "co-3d0.1", "co-3d0.2"},
	}

	got, err := ResolvePartialID(ctx, store, "co-3d0")
	if err != nil {
		t.Fatalf("ResolvePartialID(\"co-3d0\") unexpected error: %v", err)
	}
	if got != "co-3d0" {
		t.Fatalf("ResolvePartialID(\"co-3d0\") = %q, want exact match \"co-3d0\"", got)
	}
}

// The wisp fallback path has its own copy of the match filter; it must enforce
// the same prefix rule.
func TestResolvePartialID_WispPrefixNotSubstring(t *testing.T) {
	ctx := context.Background()
	store := &fakeIDStore{
		prefix: "co",
		ids:    []string{},
		wisps:  []string{"co-w7k2p"},
	}

	got, err := ResolvePartialID(ctx, store, "co-w7k")
	if err != nil {
		t.Fatalf("ResolvePartialID(\"co-w7k\") unexpected error: %v", err)
	}
	if got != "co-w7k2p" {
		t.Fatalf("ResolvePartialID(\"co-w7k\") = %q, want \"co-w7k2p\"", got)
	}

	if _, err := ResolvePartialID(ctx, store, "co-7k2"); err == nil {
		t.Fatal("ResolvePartialID(\"co-7k2\") resolved a non-prefix wisp fragment; want error")
	}
}

// ResolvePartialIDs is the batch entry point used by `bd defer` / `bd undefer`.
// A bad ID anywhere in the batch must fail the whole call rather than silently
// resolving to a substring match.
func TestResolvePartialIDs_BatchRejectsSubstring(t *testing.T) {
	ctx := context.Background()
	store := newFakeIDStore()

	if _, err := ResolvePartialIDs(ctx, store, []string{"co-fi9bx", "co-9bx"}); err == nil {
		t.Fatal("ResolvePartialIDs resolved a non-prefix fragment in a batch; want error")
	}

	got, err := ResolvePartialIDs(ctx, store, []string{"co-fi", "co-bfw"})
	if err != nil {
		t.Fatalf("ResolvePartialIDs unexpected error: %v", err)
	}
	want := []string{"co-fi9bx", "co-bfwpn"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ResolvePartialIDs = %v, want %v", got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Upstream tests for the unexported prefix normalizer, carried in from
// gastownhall/beads. The same filename was added independently on both sides;
// the two test sets are unrelated, so both are kept.
// ---------------------------------------------------------------------------

// TestParseIssueID covers the unexported prefix normalizer, so it lives in
// package utils. It is split out from the store-backed resolution tests
// because those import internal/storage/dolt, which reaches this package
// again through internal/workapi — an import cycle for an in-package test.
// The store-backed half is an external test package for that reason.

func TestParseIssueID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		prefix   string
		expected string
	}{
		{
			name:     "already has prefix",
			input:    "bd-a3f8e9",
			prefix:   "bd-",
			expected: "bd-a3f8e9",
		},
		{
			name:     "missing prefix",
			input:    "a3f8e9",
			prefix:   "bd-",
			expected: "bd-a3f8e9",
		},
		{
			name:     "hierarchical with prefix",
			input:    "bd-a3f8e9.1.2",
			prefix:   "bd-",
			expected: "bd-a3f8e9.1.2",
		},
		{
			name:     "hierarchical without prefix",
			input:    "a3f8e9.1.2",
			prefix:   "bd-",
			expected: "bd-a3f8e9.1.2",
		},
		{
			name:     "custom prefix with ID",
			input:    "ticket-123",
			prefix:   "ticket-",
			expected: "ticket-123",
		},
		{
			name:     "custom prefix without ID",
			input:    "123",
			prefix:   "ticket-",
			expected: "ticket-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseIssueID(tt.input, tt.prefix)
			if result != tt.expected {
				t.Errorf("parseIssueID(%q, %q) = %q; want %q", tt.input, tt.prefix, result, tt.expected)
			}
		})
	}
}

func TestLooksLikePrefixedID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"aap-4ar", true},
		{"bd-abc123", true},
		{"hq-xyz", true},
		{"cr-99", true},
		{"myproj-task1", true},
		{"a-b", true},        // minimal valid prefix
		{"abc12345-x", true}, // 8-char prefix (max)

		// Invalid cases
		{"abc", false},         // no hyphen
		{"", false},            // empty
		{"-abc", false},        // hyphen at start
		{"ABC-123", false},     // uppercase
		{"abcdefghi-x", false}, // prefix too long (9 chars)
		{"abc-", false},        // empty suffix
		{"abc--def", false},    // suffix starts with hyphen
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := looksLikePrefixedID(tt.input)
			if result != tt.expected {
				t.Errorf("looksLikePrefixedID(%q) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}
