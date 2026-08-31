//go:build !cgo

package goja

import "testing"

func TestPureGoBuildReportsPCRE2Requirement(t *testing.T) {
	if _, err := compileRegexpBackend(`(?<=foo)bar`, regexpBackendOptions{}); err == nil {
		t.Fatal("expected a CGO requirement error")
	}
}
