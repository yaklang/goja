# Yaklang Goja fork

This repository is a hard fork maintained for Yaklang. It combines Goja and
goja_nodejs sources in one module, targets Yaklang's Go toolchain, and
uses a bounded PCRE2 fallback without depending on `github.com/dlclark/regexp2`.

## Maintenance and licensing

The baselines below are intentionally frozen. This repository does not merge or
rebase upstream branches after the fork. Security fixes or useful language
features may be reviewed and ported as independent Yaklang changes, with their
source commit recorded, but they are not automatic upstream synchronisations.

The upstream Goja and goja_nodejs code remains under its original MIT license.
Yaklang's modifications are also released under the MIT license. See `LICENSE`
and `NOTICE` for attribution and redistribution terms.

## Upstream baselines

- Goja: `dop251/goja` commit `8f1c0696a37b221d3b14dc4c2e826cc22a6b723d`
  (2026-08-26).
- goja_nodejs: `dop251/goja_nodejs` commit
  `1f56ff5bcf1444844c756986ed5fc03069aec1ff` (2026-02-12), integrated as a
  squashed git subtree under `nodejs/`.

These provenance points must remain in this document for the lifetime of the
fork. Any selectively ported upstream change must be listed in the release
notes and must rerun the full root-module test matrix.

## Module and toolchain policy

- Module path: `github.com/yaklang/goja`.
- Minimum Go version: 1.22.12, matching Yaklang.
- NodeJS packages share the root `go.mod`; `nodejs/` intentionally has no
  nested module files.
- The production module graph must not contain `github.com/dlclark/regexp2`.

## Regular-expression backend

The standard library `regexp` engine remains the fast path for compatible
patterns. `github.com/VillanCh/go-pcre2-lite` provides the only backtracking
backend for lookaround, backreferences, non-zero search starts, and other
ECMAScript cases that RE2 cannot handle. PCRE2 10.47 is embedded in that module,
JIT is disabled, and match/depth limits bound catastrophic backtracking.

The PCRE2 binding uses its 8-bit UTF mode, while JavaScript strings are UTF-16.
To preserve lone-surrogate behavior without a second regex engine, this fork
maps isolated surrogate code units one-to-one into U+10F800..U+10FFFF before
matching and rewrites corresponding pattern escapes. Indexes remain UTF-16 code
unit indexes. The reserved mapping range is the one intentional edge-case
tradeoff: a source string containing both an isolated surrogate and an actual
scalar in that final supplementary private-use range cannot be represented
injectively by the 8-bit backend.

`CGO_ENABLED=1` is required when a script needs the PCRE2 fallback. A no-cgo
build is kept compilable for dependency analysis and tooling, but fallback-only
patterns return a compile error at runtime.

### Regular-expression changes from upstream

- Removed every source and module dependency on `github.com/dlclark/regexp2`.
- Kept Go's RE2 engine as the zero-cgo fast path for compatible expressions.
- Added `go-pcre2-lite` v0.1.6 with embedded PCRE2 10.47 as the only
  backtracking engine; no system PCRE2 installation is used and JIT is disabled.
- Added PCRE2 callout-based ECMAScript lookbehind handling for right-to-left
  captures, variable-length alternatives, nested assertions, and
  internal/external/forward/mutual backreferences.
- Preserved UTF-16 indexes and isolated surrogate behavior over PCRE2's 8-bit
  UTF interface using the mapping described above.
- Preserved PCRE2 match and depth limits, so the fallback remains bounded for
  adversarial backtracking inputs.
- Cached identical regexp literals per compiled Program by `(pattern, flags)`;
  each JavaScript RegExp object still has independent mutable state such as
  `lastIndex`.

The compatibility implementation is primarily in `regexp.go`,
`regexp_pcre2.go`, `regexp_nocgo.go`, `builtin_regexp.go`, and the compiler's
regexp-literal cache. The lower-level ECMAScript lookbehind bridge is maintained
in `github.com/VillanCh/go-pcre2-lite/regexp2/ecma_lookbehind.go`.

## Go 1.22 weak collections

The latest upstream implementation uses Go 1.25's `weak` package and
`runtime.AddCleanup`. Go 1.22 cannot reproduce those garbage-collector hooks.
This fork stores WeakMap and WeakSet entries on their key objects under a
per-collection ID. Observable JavaScript operations remain compatible, but a
value can stay reachable while its key stays reachable after the collection
itself has been collected. Revisit this compatibility layer when Yaklang raises
its minimum Go version to 1.25 or newer.
