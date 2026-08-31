//go:build !cgo

package goja

import "errors"

var errPCRE2RequiresCGO = errors.New("goja: PCRE2 regular expressions require CGO_ENABLED=1")

func compileRegexpBackend(src string, opts regexpBackendOptions) (regexpBackend, error) {
	return nil, errPCRE2RequiresCGO
}
