# Yaklang Goja fork

This repository is a hard fork maintained for Yaklang. It combines current
Goja and goja_nodejs sources in one module, targets Yaklang's Go toolchain, and
uses a bounded PCRE2 fallback without depending on `github.com/dlclark/regexp2`.

## Upstream baselines

- Goja: `dop251/goja` commit `8f1c0696a37b221d3b14dc4c2e826cc22a6b723d`
  (2026-08-26).
- goja_nodejs: `dop251/goja_nodejs` commit
  `1f56ff5bcf1444844c756986ed5fc03069aec1ff` (2026-02-12), integrated as a
  squashed git subtree under `nodejs/`.

Future upstream syncs should preserve these two provenance points in the merge
or subtree commit message and rerun the full root-module test matrix.

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

## Go 1.22 weak collections

The latest upstream implementation uses Go 1.25's `weak` package and
`runtime.AddCleanup`. Go 1.22 cannot reproduce those garbage-collector hooks.
This fork stores WeakMap and WeakSet entries on their key objects under a
per-collection ID. Observable JavaScript operations remain compatible, but a
value can stay reachable while its key stays reachable after the collection
itself has been collected. Revisit this compatibility layer when Yaklang raises
its minimum Go version to 1.25 or newer.
