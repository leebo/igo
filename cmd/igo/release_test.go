package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in   string
		want version
		bad  bool
	}{
		{"v1.2.3", version{1, 2, 3}, false},
		{"1.2.3", version{1, 2, 3}, false},
		{"v0.0.0", version{0, 0, 0}, false},
		{"v10.20.30", version{10, 20, 30}, false},
		{"", version{}, true},
		{"v1", version{}, true},
		{"v1.2", version{}, true},
		{"v1.2.3-rc1", version{}, true},
		{"foo", version{}, true},
	}
	for _, tc := range cases {
		got, err := parseVersion(tc.in)
		if tc.bad {
			if err == nil {
				t.Errorf("parseVersion(%q) = %v, want error", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("parseVersion(%q) = %v, %v; want %v, nil", tc.in, got, err, tc.want)
		}
	}
}

func TestApplyBump(t *testing.T) {
	cases := []struct {
		in   version
		lvl  string
		want version
	}{
		{version{1, 2, 3}, "patch", version{1, 2, 4}},
		{version{1, 2, 3}, "minor", version{1, 3, 0}},
		{version{1, 2, 3}, "major", version{2, 0, 0}},
		{version{0, 1, 0}, "minor", version{0, 2, 0}},
		{version{0, 0, 0}, "patch", version{0, 0, 1}},
	}
	for _, tc := range cases {
		if got := applyBump(tc.in, tc.lvl); got != tc.want {
			t.Errorf("applyBump(%v, %q) = %v; want %v", tc.in, tc.lvl, got, tc.want)
		}
	}
}

func TestParseCommit(t *testing.T) {
	cases := []struct {
		hash, msg string
		wantType  string
		wantScope string
		wantBreak bool
		wantSub   string
	}{
		{"abc123", "feat: add foo", "feat", "", false, "add foo"},
		{"abc123", "feat!: add foo", "feat", "", true, "add foo"},
		{"abc123", "feat(api): add foo", "feat", "api", false, "add foo"},
		{"abc123", "feat(api)!: add foo", "feat", "api", true, "add foo"},
		{"abc123", "fix: bar\n\nBREAKING CHANGE: see migration", "fix", "", true, "bar"},
		{"abc123", "chore(release): v1.0.0", "chore", "release", false, "v1.0.0"},
		{"abc123", "Add README", "", "", false, "Add README"}, // non-conv falls through
		{"abc123", "refactor!: drop X", "refactor", "", true, "drop X"},
	}
	for _, tc := range cases {
		got := parseCommit(tc.hash, tc.msg)
		if got.Type != tc.wantType || got.Scope != tc.wantScope ||
			got.Breaking != tc.wantBreak || got.Subject != tc.wantSub {
			t.Errorf("parseCommit(%q) = %+v; want type=%q scope=%q break=%v sub=%q",
				tc.msg, got, tc.wantType, tc.wantScope, tc.wantBreak, tc.wantSub)
		}
	}
}

func TestDecideBump_Pre1(t *testing.T) {
	prev := version{0, 1, 0}
	cases := []struct {
		name string
		cs   []commit
		want bumpLevel
	}{
		{"only chores", []commit{{Type: "chore"}, {Type: "test"}}, bumpNone},
		{"feat in 0.x", []commit{{Type: "feat"}}, bumpPatch},
		{"fix in 0.x", []commit{{Type: "fix"}}, bumpPatch},
		{"breaking in 0.x bumps minor", []commit{{Type: "refactor", Breaking: true}}, bumpMinor},
		{"feat + chore in 0.x", []commit{{Type: "feat"}, {Type: "chore"}}, bumpPatch},
		{"breaking wins over feat", []commit{{Type: "feat"}, {Type: "fix", Breaking: true}}, bumpMinor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := decideBump(tc.cs, prev); got != tc.want {
				t.Errorf("decideBump = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestDecideBump_Post1(t *testing.T) {
	prev := version{1, 0, 0}
	cases := []struct {
		name string
		cs   []commit
		want bumpLevel
	}{
		{"feat post-1 bumps minor", []commit{{Type: "feat"}}, bumpMinor},
		{"breaking post-1 bumps major", []commit{{Type: "feat", Breaking: true}}, bumpMajor},
		{"fix only post-1 bumps patch", []commit{{Type: "fix"}}, bumpPatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := decideBump(tc.cs, prev); got != tc.want {
				t.Errorf("decideBump = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestRenderChangelogEntry(t *testing.T) {
	commits := []commit{
		{Hash: "1234567890abc", Type: "feat", Subject: "add x"},
		{Hash: "234567890abcd", Type: "fix", Scope: "core", Subject: "y is wrong"},
		{Hash: "abcdef1234567", Type: "refactor", Breaking: true, Subject: "drop legacy"},
		{Hash: "0000000000000", Type: "chore", Subject: "tidy"},
	}
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	out := renderChangelogEntry(version{0, 2, 0}, "v0.1.0", commits, now)

	for _, want := range []string{
		"## [v0.2.0] - 2026-05-02",
		"_Changes since v0.1.0_",
		"### ⚠️ BREAKING CHANGES",
		"drop legacy (abcdef1)",
		"### Features",
		"add x (1234567)",
		"### Bug Fixes",
		"**core:** y is wrong (2345678)",
		"### Other",
		"tidy (0000000)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestPrependChangelog_HeaderBlankLine(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/CHANGELOG.md"
	// header without trailing blank line — first release scenario
	if err := writeFile(path, "# Changelog\n\nIntro.\n"); err != nil {
		t.Fatal(err)
	}
	entry := "## [v0.1.0] - 2026-05-02\n\n### Features\n\n- thing\n"
	if err := prependChangelog(path, entry); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)
	want := "# Changelog\n\nIntro.\n\n## [v0.1.0] - 2026-05-02\n\n### Features\n\n- thing\n\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func writeFile(p, c string) error {
	return os.WriteFile(p, []byte(c), 0o644)
}

func readFile(t *testing.T, p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSplitChangelog(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		wantHeader string
		wantBody   string
	}{
		{
			name:       "empty file gets trailing newline",
			input:      "# Changelog",
			wantHeader: "# Changelog\n",
			wantBody:   "",
		},
		{
			name:       "no prior versions",
			input:      "# Changelog\n\nIntro paragraph.\n",
			wantHeader: "# Changelog\n\nIntro paragraph.\n",
			wantBody:   "",
		},
		{
			name:       "split before first version section",
			input:      "# Changelog\n\nIntro.\n\n## [v0.1.0] - 2026-01-01\n\nThings.\n",
			wantHeader: "# Changelog\n\nIntro.\n\n",
			wantBody:   "## [v0.1.0] - 2026-01-01\n\nThings.\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, b := splitChangelog(tc.input)
			if h != tc.wantHeader || b != tc.wantBody {
				t.Errorf("splitChangelog\n  header got=%q want=%q\n  body got=%q want=%q",
					h, tc.wantHeader, b, tc.wantBody)
			}
		})
	}
}
