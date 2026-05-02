package core

import (
	"testing"
)

func TestDetectMode(t *testing.T) {
	cases := []struct {
		env  string
		want Mode
	}{
		{"", ModeDev},
		{"dev", ModeDev},
		{"DEV", ModeDev},
		{"development", ModeDev},
		{"unknown", ModeDev},
		{"test", ModeTest},
		{"Testing", ModeTest},
		{"prd", ModePrd},
		{"prod", ModePrd},
		{"PRODUCTION", ModePrd},
		{"  prd  ", ModePrd},
	}
	for _, tc := range cases {
		t.Setenv("IGO_ENV", tc.env)
		if got := DetectMode(); got != tc.want {
			t.Errorf("IGO_ENV=%q: got %q, want %q", tc.env, got, tc.want)
		}
	}
}

func TestModePredicates(t *testing.T) {
	if !ModeDev.IsDev() || ModeDev.IsTest() || ModeDev.IsPrd() {
		t.Error("ModeDev predicates wrong")
	}
	if !ModeTest.IsTest() || ModeTest.IsDev() || ModeTest.IsPrd() {
		t.Error("ModeTest predicates wrong")
	}
	if !ModePrd.IsPrd() || ModePrd.IsDev() || ModePrd.IsTest() {
		t.Error("ModePrd predicates wrong")
	}
}
