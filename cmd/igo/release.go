package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// runRelease 是 `igo release [flags]` 子命令入口。
//
// 行为概述:
//  1. 拒绝 dirty working tree
//  2. 找最近的 v* tag (没有则用 root commit)
//  3. 收集 since→HEAD 的 commits,按 Conventional Commits 分类
//  4. 根据 0.x 规则决定 bump:BREAKING→minor, feat/fix/perf→patch
//  5. 生成 CHANGELOG.md 条目 (prepend),commit
//  6. 打 annotated tag vX.Y.Z
//  7. --push: 推 branch + tag; 默认不 push
//
// 关键 flags: --bump, --tag, --from, --changelog, --push, --dry-run
func runRelease(args []string) int {
	fs := flag.NewFlagSet("release", flag.ContinueOnError)
	bump := fs.String("bump", "", "override auto-detected bump: patch|minor|major")
	tagOverride := fs.String("tag", "", "override the computed version (must start with v)")
	from := fs.String("from", "", "ref to compute changes from (default: latest v* tag)")
	changelogPath := fs.String("changelog", "CHANGELOG.md", "path to the changelog file")
	push := fs.Bool("push", false, "push the release commit and tag to origin")
	dryRun := fs.Bool("dry-run", false, "print preview without modifying files / git state")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if !*dryRun {
		if dirty, err := isWorkTreeDirty(); err != nil || dirty {
			if err != nil {
				fmt.Fprintf(os.Stderr, "[igo release] git status: %v\n", err)
			} else {
				fmt.Fprintln(os.Stderr, "[igo release] working tree is dirty; commit or stash first")
			}
			return 1
		}
	}

	prevTag := *from
	if prevTag == "" {
		t, _ := latestVersionTag()
		prevTag = t
	}

	commits, err := loadCommits(prevTag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[igo release] git log: %v\n", err)
		return 1
	}
	if len(commits) == 0 {
		fmt.Fprintln(os.Stderr, "[igo release] no commits since last tag; nothing to release")
		return 1
	}

	prevVersion, _ := parseVersion(prevTag)
	var nextVersion version
	switch {
	case *tagOverride != "":
		v, err := parseVersion(*tagOverride)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[igo release] invalid --tag: %v\n", err)
			return 1
		}
		nextVersion = v
	case *bump != "":
		nextVersion = applyBump(prevVersion, *bump)
	default:
		level := decideBump(commits, prevVersion)
		if level == bumpNone {
			fmt.Fprintln(os.Stderr, "[igo release] all commits are non-release-worthy (chore/test/ci/docs);")
			fmt.Fprintln(os.Stderr, "  pass --bump patch|minor|major to force a release")
			return 1
		}
		nextVersion = applyBump(prevVersion, string(level))
	}

	if exists, _ := tagExists("v" + nextVersion.String()); exists {
		fmt.Fprintf(os.Stderr, "[igo release] tag v%s already exists\n", nextVersion.String())
		return 1
	}

	entry := renderChangelogEntry(nextVersion, prevTag, commits, time.Now())

	if *dryRun {
		fmt.Println("=== [DRY RUN] would prepend to", *changelogPath, "===")
		fmt.Println(entry)
		fmt.Printf("=== [DRY RUN] would commit + tag v%s ===\n", nextVersion.String())
		if *push {
			fmt.Println("=== [DRY RUN] would push commit + tag ===")
		}
		return 0
	}

	if err := prependChangelog(*changelogPath, entry); err != nil {
		fmt.Fprintf(os.Stderr, "[igo release] write changelog: %v\n", err)
		return 1
	}

	tagName := "v" + nextVersion.String()
	if err := gitRun("add", *changelogPath); err != nil {
		return 1
	}
	if err := gitRun("commit", "-m", fmt.Sprintf("chore(release): %s", tagName)); err != nil {
		return 1
	}
	if err := gitRun("tag", "-a", tagName, "-m", entry); err != nil {
		return 1
	}
	fmt.Printf("[igo release] created %s\n", tagName)

	if *push {
		branch, err := currentBranch()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[igo release] resolve branch: %v\n", err)
			return 1
		}
		if err := gitRun("push", "origin", branch); err != nil {
			return 1
		}
		if err := gitRun("push", "origin", tagName); err != nil {
			return 1
		}
		fmt.Printf("[igo release] pushed %s and %s\n", branch, tagName)
	} else {
		fmt.Println("[igo release] skipped push; run `git push origin <branch> && git push origin", tagName, "` when ready")
	}
	return 0
}

// ---------- version ----------

type version struct{ Major, Minor, Patch int }

func (v version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

var versionRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

func parseVersion(s string) (version, error) {
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return version{}, fmt.Errorf("not a semver: %q", s)
	}
	mj, _ := strconv.Atoi(m[1])
	mn, _ := strconv.Atoi(m[2])
	pt, _ := strconv.Atoi(m[3])
	return version{mj, mn, pt}, nil
}

type bumpLevel string

const (
	bumpNone  bumpLevel = "none"
	bumpPatch bumpLevel = "patch"
	bumpMinor bumpLevel = "minor"
	bumpMajor bumpLevel = "major"
)

func applyBump(v version, lvl string) version {
	switch bumpLevel(lvl) {
	case bumpMajor:
		return version{v.Major + 1, 0, 0}
	case bumpMinor:
		return version{v.Major, v.Minor + 1, 0}
	default: // patch
		return version{v.Major, v.Minor, v.Patch + 1}
	}
}

// decideBump applies the project's policy: while in 0.x, BREAKING bumps minor
// (instead of major) and feat/fix/perf bump patch. Once at v1+, BREAKING bumps
// major and feat bumps minor (standard SemVer).
func decideBump(commits []commit, prev version) bumpLevel {
	preV1 := prev.Major == 0
	hasBreaking, hasFeat, hasFix := false, false, false
	for _, c := range commits {
		if c.Breaking {
			hasBreaking = true
		}
		switch c.Type {
		case "feat":
			hasFeat = true
		case "fix", "perf":
			hasFix = true
		}
	}
	switch {
	case hasBreaking && !preV1:
		return bumpMajor
	case hasBreaking && preV1:
		return bumpMinor
	case hasFeat && !preV1:
		return bumpMinor
	case hasFeat || hasFix:
		return bumpPatch
	default:
		return bumpNone
	}
}

// ---------- commits ----------

type commit struct {
	Hash     string
	Type     string // feat, fix, ...
	Scope    string
	Breaking bool
	Subject  string
}

// commitRe captures: type(scope)?!?: subject
var commitRe = regexp.MustCompile(`^(\w+)(?:\(([^)]+)\))?(!)?:\s+(.+)$`)

func parseCommit(hash, fullMsg string) commit {
	subject := strings.SplitN(fullMsg, "\n", 2)[0]
	c := commit{Hash: hash, Subject: strings.TrimSpace(subject)}
	if m := commitRe.FindStringSubmatch(c.Subject); m != nil {
		c.Type = m[1]
		c.Scope = m[2]
		c.Breaking = m[3] == "!"
		c.Subject = strings.TrimSpace(m[4])
	}
	if strings.Contains(fullMsg, "BREAKING CHANGE:") || strings.Contains(fullMsg, "BREAKING-CHANGE:") {
		c.Breaking = true
	}
	return c
}

func loadCommits(sinceRef string) ([]commit, error) {
	args := []string{"log", "--no-merges", "--format=%H%x00%B%x1e"}
	if sinceRef != "" {
		args = append(args, sinceRef+"..HEAD")
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, err
	}
	var out2 []commit
	for _, rec := range strings.Split(string(out), "\x1e") {
		rec = strings.TrimLeft(rec, "\n")
		if rec == "" {
			continue
		}
		parts := strings.SplitN(rec, "\x00", 2)
		if len(parts) != 2 {
			continue
		}
		out2 = append(out2, parseCommit(parts[0], parts[1]))
	}
	return out2, nil
}

// ---------- changelog ----------

// renderChangelogEntry groups commits by section and returns a Markdown block
// suitable for prepending under the file header.
func renderChangelogEntry(next version, prevTag string, commits []commit, now time.Time) string {
	var breaking, feats, fixes, perfs, refactors, others []commit
	for _, c := range commits {
		switch {
		case c.Breaking:
			breaking = append(breaking, c)
		case c.Type == "feat":
			feats = append(feats, c)
		case c.Type == "fix":
			fixes = append(fixes, c)
		case c.Type == "perf":
			perfs = append(perfs, c)
		case c.Type == "refactor":
			refactors = append(refactors, c)
		default:
			others = append(others, c)
		}
	}

	var b strings.Builder
	tagName := "v" + next.String()
	fmt.Fprintf(&b, "## [%s] - %s\n\n", tagName, now.Format("2006-01-02"))
	if prevTag != "" {
		fmt.Fprintf(&b, "_Changes since %s_\n\n", prevTag)
	}
	writeSection(&b, "⚠️ BREAKING CHANGES", breaking)
	writeSection(&b, "Features", feats)
	writeSection(&b, "Bug Fixes", fixes)
	writeSection(&b, "Performance", perfs)
	writeSection(&b, "Refactors", refactors)
	writeSection(&b, "Other", others)
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func writeSection(b *strings.Builder, title string, cs []commit) {
	if len(cs) == 0 {
		return
	}
	// keep a stable order: by type/scope/subject so equal inputs yield equal output
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].Subject < cs[j].Subject })
	fmt.Fprintf(b, "### %s\n\n", title)
	for _, c := range cs {
		short := c.Hash
		if len(short) > 7 {
			short = short[:7]
		}
		prefix := ""
		if c.Scope != "" {
			prefix = "**" + c.Scope + ":** "
		}
		fmt.Fprintf(b, "- %s%s (%s)\n", prefix, c.Subject, short)
	}
	b.WriteString("\n")
}

const changelogHeader = `# Changelog

All notable changes to this project will be documented in this file.

The format follows [Conventional Commits](https://www.conventionalcommits.org/)
and [Semantic Versioning](https://semver.org/). While in v0.x, BREAKING
changes bump MINOR per the 0.x convention.

`

func prependChangelog(path, entry string) error {
	existing, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		existing = []byte(changelogHeader)
		err = nil
	}
	if err != nil {
		return err
	}
	header, body := splitChangelog(string(existing))
	merged := header + entry + "\n" + body
	return os.WriteFile(path, []byte(merged), 0o644)
}

// splitChangelog separates the file into a header (everything up to the first
// `## [` line) and body (the existing version sections).
func splitChangelog(s string) (header, body string) {
	idx := strings.Index(s, "\n## [")
	if idx < 0 {
		// no prior version section; entire file is header
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		return s, ""
	}
	return s[:idx+1], s[idx+1:]
}

// ---------- git helpers ----------

func gitRun(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[igo release] git %s failed: %v\n", strings.Join(args, " "), err)
		return err
	}
	return nil
}

func isWorkTreeDirty() (bool, error) {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func latestVersionTag() (string, error) {
	out, err := exec.Command("git", "tag", "--list", "v*", "--sort=-v:refname").Output()
	if err != nil {
		return "", err
	}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if _, err := parseVersion(line); err == nil {
			return line, nil
		}
	}
	return "", nil
}

func tagExists(tag string) (bool, error) {
	out, err := exec.Command("git", "tag", "--list", tag).Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func currentBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
