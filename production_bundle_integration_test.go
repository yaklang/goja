package goja

import (
	"os"
	"testing"
)

// TestCompileExternalProductionBundle provides a reproducible compile-only
// check for a real minified application bundle without committing that bundle
// to this repository. Set GOJA_PRODUCTION_BUNDLE to its path to enable it.
func TestCompileExternalProductionBundle(t *testing.T) {
	path := os.Getenv("GOJA_PRODUCTION_BUNDLE")
	if path == "" {
		t.Skip("GOJA_PRODUCTION_BUNDLE is not set")
	}
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Compile(path, string(source), false); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkCompileExternalProductionBundle(b *testing.B) {
	path := os.Getenv("GOJA_PRODUCTION_BUNDLE")
	if path == "" {
		b.Skip("GOJA_PRODUCTION_BUNDLE is not set")
	}
	source, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err = Compile(path, string(source), false); err != nil {
			b.Fatal(err)
		}
	}
}
