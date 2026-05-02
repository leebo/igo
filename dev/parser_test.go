package dev

import (
	"testing"
)

func TestParseBuildErrors(t *testing.T) {
	stderr := `# trial
./handlers/user.go:42:10: undefined: foo
./main.go:5:8: "fmt" imported and not used
./handlers/book.go:11:2: cannot use 1 (untyped int constant) as string value in assignment
./parser.go:18:1: syntax error: unexpected }
go: finding module for package github.com/missing/pkg
`
	got := ParseBuildErrors(stderr)
	if len(got) != 4 {
		t.Fatalf("expected 4 parsed errors, got %d: %+v", len(got), got)
	}

	if got[0].Type != ErrorTypeUndefined || got[0].Symbol != "foo" || got[0].Line != 42 || got[0].Col != 10 {
		t.Errorf("undefined parse wrong: %+v", got[0])
	}
	if got[1].Type != ErrorTypeMissingImport {
		t.Errorf("missing-import parse wrong: %+v", got[1])
	}
	if got[2].Type != ErrorTypeTypeMismatch {
		t.Errorf("type-mismatch parse wrong: %+v", got[2])
	}
	if got[3].Type != ErrorTypeSyntax {
		t.Errorf("syntax parse wrong: %+v", got[3])
	}
	for _, e := range got {
		if e.File == "" || e.Line <= 0 {
			t.Errorf("missing location: %+v", e)
		}
	}
}

func TestParseBuildErrors_Empty(t *testing.T) {
	if got := ParseBuildErrors(""); len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
	if got := ParseBuildErrors("# package\ngo: ok\n"); len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}
