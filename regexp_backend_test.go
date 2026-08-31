package goja

import "testing"

func TestBacktrackingBackendDoesNotExposeNumericCaptureNames(t *testing.T) {
	wrapper, err := compileBacktrackingRegexp(`t(e)(st(\d?))`, false, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	results := wrapper.findAllSubmatchIndex(newStringValue("test1test2"), 0, -1, false, false)
	if len(results) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(results))
	}
	for resultIndex, result := range results {
		for groupIndex, name := range result.groups {
			if name != "" {
				t.Fatalf("match %d group %d unexpectedly has name %q", resultIndex, groupIndex, name)
			}
		}
	}
}

func TestBacktrackingBackendPreservesNamedCapture(t *testing.T) {
	wrapper, err := compileBacktrackingRegexp(`(?<word>foo)(bar)`, false, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	result := wrapper.findSubmatchIndex(newStringValue("foobar"), 0, false, false)
	if len(result.groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(result.groups))
	}
	if result.groups[1] != "word" || result.groups[2] != "" {
		t.Fatalf("unexpected group names: %#v", result.groups)
	}
}
