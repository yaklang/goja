//go:build cgo

package goja

import (
	"strings"
	"testing"
)

func TestPCRE2BackendIsUsedForBacktrackingPatterns(t *testing.T) {
	backend, err := compileRegexpBackend(`(?<=foo)bar`, regexpBackendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := backend.(*pcre2RegexpBackend); !ok {
		t.Fatalf("expected PCRE2 backend, got %T", backend)
	}
	match, err := backend.FindRunesMatchStartingAt([]rune("foobar"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if match == nil || match.Groups()[0].index != 3 {
		t.Fatalf("unexpected match: %#v", match)
	}
}

func TestPCRE2BackendMapsIsolatedSurrogates(t *testing.T) {
	backend, err := compileRegexpBackend(`(?=\uD800)`, regexpBackendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	match, err := backend.FindRunesMatchStartingAt([]rune{0xD800}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if match == nil {
		t.Fatal("expected an isolated surrogate match")
	}
	groups := match.Groups()
	if groups[0].index != 0 || groups[0].length != 0 {
		t.Fatalf("unexpected zero-width match: %#v", groups[0])
	}
}

func TestRewriteSurrogateEscapes(t *testing.T) {
	const pattern = `[\uD800-\uDFFF]\\uD800`
	const expected = "[\U0010f800-\U0010ffff]\\\\uD800"
	if actual := rewriteSurrogateEscapes(pattern); actual != expected {
		t.Fatalf("expected %q, got %q", expected, actual)
	}
}

func BenchmarkPCRE2BacktrackingShort(b *testing.B) {
	backend, err := compileRegexpBackend(`(a+)\1`, regexpBackendOptions{})
	if err != nil {
		b.Fatal(err)
	}
	input := []rune("aaaaaaaaaaaaaaaa")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		match, err := backend.FindRunesMatchStartingAt(input, 0)
		if err != nil || match == nil {
			b.Fatalf("match failed: %v", err)
		}
	}
}

func BenchmarkPCRE2BacktrackingLargeInput(b *testing.B) {
	backend, err := compileRegexpBackend(`(?<=contact )[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`, regexpBackendOptions{})
	if err != nil {
		b.Fatal(err)
	}
	input := []rune(strings.Repeat("the quick brown fox jumps over the lazy dog. ", 2300) + "contact needle@host.example.org")
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		match, err := backend.FindRunesMatchStartingAt(input, 0)
		if err != nil || match == nil {
			b.Fatalf("match failed: %v", err)
		}
	}
}
