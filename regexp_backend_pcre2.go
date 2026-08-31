//go:build cgo

package goja

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"

	pcre2 "github.com/VillanCh/go-pcre2-lite/regexp2"
)

// PCRE2's 8-bit UTF mode cannot represent isolated UTF-16 surrogates. Map
// each surrogate code unit one-to-one into the final 2,048 code points of the
// supplementary private-use area. Match indexes remain unchanged because the
// mapping is one rune per UTF-16 code unit.
const surrogatePlaceholderBase rune = 0x10F800

type pcre2RegexpBackend struct {
	rx *pcre2.Regexp
}

type pcre2RegexpMatch struct {
	match *pcre2.Match
}

func compileRegexpBackend(src string, opts regexpBackendOptions) (regexpBackend, error) {
	var options pcre2.RegexOptions = pcre2.ECMAScript
	if opts.multiline {
		options |= pcre2.Multiline
	}
	if opts.dotAll {
		options |= pcre2.Singleline
	}
	if opts.ignoreCase {
		options |= pcre2.IgnoreCase
	}
	if opts.unicode {
		options |= pcre2.Unicode
	}
	src = rewriteSurrogateEscapes(src)
	rx, err := pcre2.Compile(src, options)
	if err != nil {
		return nil, fmt.Errorf("pcre2: %w", err)
	}
	return &pcre2RegexpBackend{rx: rx}, nil
}

func (r *pcre2RegexpBackend) FindRunesMatchStartingAt(input []rune, start int) (regexpBackendMatch, error) {
	match, err := r.rx.FindRunesMatchStartingAt(mapSurrogateRunes(input), start)
	if match == nil || err != nil {
		return nil, err
	}
	return &pcre2RegexpMatch{match: match}, nil
}

func (r *pcre2RegexpBackend) FindNextMatch(previous regexpBackendMatch) (regexpBackendMatch, error) {
	match, err := r.rx.FindNextMatch(previous.(*pcre2RegexpMatch).match)
	if match == nil || err != nil {
		return nil, err
	}
	return &pcre2RegexpMatch{match: match}, nil
}

func (m *pcre2RegexpMatch) Groups() []regexpBackendGroup {
	groups := m.match.Groups()
	result := make([]regexpBackendGroup, len(groups))
	for i, group := range groups {
		name := group.Name
		if name == strconv.Itoa(i) {
			name = ""
		}
		result[i] = regexpBackendGroup{
			index:   group.Index,
			length:  group.Length,
			name:    name,
			matched: len(group.Captures) > 0,
		}
	}
	return result
}

func mapSurrogateRunes(input []rune) []rune {
	for index, value := range input {
		if utf16.IsSurrogate(value) {
			mapped := append([]rune(nil), input...)
			for i := index; i < len(mapped); i++ {
				if utf16.IsSurrogate(mapped[i]) {
					mapped[i] = surrogatePlaceholderBase + mapped[i] - 0xD800
				}
			}
			return mapped
		}
	}
	return input
}

func rewriteSurrogateEscapes(pattern string) string {
	var rewritten strings.Builder
	last := 0
	for index := 0; index < len(pattern); {
		if pattern[index] != '\\' {
			index++
			continue
		}
		runStart := index
		for index < len(pattern) && pattern[index] == '\\' {
			index++
		}
		if (index-runStart)%2 == 0 || index+5 > len(pattern) || pattern[index] != 'u' {
			continue
		}
		value, ok := decodeHex(pattern[index+1 : index+5])
		if !ok || value < 0xD800 || value > 0xDFFF {
			continue
		}
		rewritten.WriteString(pattern[last : index-1])
		rewritten.WriteRune(surrogatePlaceholderBase + rune(value) - 0xD800)
		index += 5
		last = index
	}
	if last == 0 {
		return pattern
	}
	rewritten.WriteString(pattern[last:])
	return rewritten.String()
}
