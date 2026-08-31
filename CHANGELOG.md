# Changelog

## v0.1.0

First supported Yaklang hard-fork release.

- Freeze Goja at upstream commit `8f1c0696a37b221d3b14dc4c2e826cc22a6b723d`.
- Integrate goja_nodejs commit `1f56ff5bcf1444844c756986ed5fc03069aec1ff`
  into the root module under `nodejs/`.
- Change the module path to `github.com/yaklang/goja` and target Go 1.22.12.
- Remove `github.com/dlclark/regexp2` completely from source and module graphs.
- Use Go RE2 as the fast path and `go-pcre2-lite` v0.1.6 with embedded PCRE2
  10.47 as the only bounded backtracking fallback.
- Restore ECMAScript lookbehind capture and backreference behavior through the
  PCRE2 callout compatibility layer without skipping Test262 cases.
- Preserve JavaScript UTF-16 indexing and isolated-surrogate behavior.
- Cache identical regexp literals within compiled Programs while preserving
  independent RegExp object state.
- Add Linux, macOS, Windows, 386, race, no-cgo compilation, static analysis,
  Test262, and nodejs type-definition CI coverage.
- Document the Go 1.22 weak-collection compatibility tradeoff and production
  bundle benchmark results.
