package goja

import (
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/yaklang/goja/unistring"
)

type backtrackingMatchCache struct {
	target String
	runes  []rune
	posMap []int
}

// Not goroutine-safe. Use backtrackingWrapper.clone()
type backtrackingWrapper struct {
	rx    regexpBackend
	cache *backtrackingMatchCache
}

type regexpWrapper regexp.Regexp

type positionMapItem struct {
	src, dst int
}
type positionMap []positionMapItem

func (m positionMap) get(src int) int {
	if src <= 0 {
		return src
	}
	res := sort.Search(len(m), func(n int) bool { return m[n].src >= src })
	if res >= len(m) || m[res].src != src {
		panic("index not found")
	}
	return m[res].dst
}

type arrayRuneReader struct {
	runes []rune
	pos   int
}

func (rd *arrayRuneReader) ReadRune() (r rune, size int, err error) {
	if rd.pos < len(rd.runes) {
		r = rd.runes[rd.pos]
		size = 1
		rd.pos++
	} else {
		err = io.EOF
	}
	return
}

// Not goroutine-safe. Use regexpPattern.clone()
type regexpPattern struct {
	src string

	global, ignoreCase, multiline, dotAll, sticky, unicode bool

	regexpWrapper       *regexpWrapper
	backtrackingWrapper *backtrackingWrapper
}

type regexpResult struct {
	indexes []int
	groups  []string
}

func (result *regexpResult) appendIndexesAndGroup(index1, index2 int, name string) {
	result.indexes = append(result.indexes, index1, index2)
	result.groups = append(result.groups, name)
}

func submatchesToRegexpResults(submatches [][]int, groups []string) []regexpResult {
	results := make([]regexpResult, 0, len(submatches))
	for _, val := range submatches {
		results = append(results, regexpResult{indexes: val, groups: groups})
	}
	return results
}

func compileBacktrackingRegexp(src string, multiline, dotAll, ignoreCase, unicode bool) (*backtrackingWrapper, error) {
	backend, err := compileRegexpBackend(src, regexpBackendOptions{
		multiline:  multiline,
		dotAll:     dotAll,
		ignoreCase: ignoreCase,
		unicode:    unicode,
	})
	if err != nil {
		return nil, err
	}
	return &backtrackingWrapper{rx: backend}, nil
}

func (p *regexpPattern) createBacktracking() {
	if p.backtrackingWrapper != nil {
		return
	}
	rx, err := compileBacktrackingRegexp(p.src, p.multiline, p.dotAll, p.ignoreCase, p.unicode)
	if err != nil {
		// At this point the expression already compiled through the RE2 path;
		// failure to compile its PCRE2 form is an adapter bug.
		panic(err)
	}
	p.backtrackingWrapper = rx
}

func buildUTF8PosMap(s unicodeString) (positionMap, string) {
	pm := make(positionMap, 0, s.Length())
	rd := s.Reader()
	sPos, utf8Pos := 0, 0
	var sb strings.Builder
	for {
		r, size, err := rd.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			// the string contains invalid UTF-16, bailing out
			return nil, ""
		}
		utf8Size, _ := sb.WriteRune(r)
		sPos += size
		utf8Pos += utf8Size
		pm = append(pm, positionMapItem{src: utf8Pos, dst: sPos})
	}
	return pm, sb.String()
}

func (p *regexpPattern) findSubmatchIndex(s String, start int) regexpResult {
	if p.regexpWrapper == nil {
		return p.backtrackingWrapper.findSubmatchIndex(s, start, p.unicode, p.global || p.sticky)
	}
	if start != 0 {
		// Unfortunately Go's regexp library does not allow starting from an arbitrary position.
		// If we just drop the first _start_ characters of the string the assertions (^, $, \b and \B) will not
		// work correctly.
		p.createBacktracking()
		return p.backtrackingWrapper.findSubmatchIndex(s, start, p.unicode, p.global || p.sticky)
	}
	return p.regexpWrapper.findSubmatchIndex(s, p.unicode)
}

func (p *regexpPattern) findAllSubmatchIndex(s String, start int, limit int, sticky bool) []regexpResult {
	if p.regexpWrapper == nil {
		return p.backtrackingWrapper.findAllSubmatchIndex(s, start, limit, sticky, p.unicode)
	}
	if start == 0 {
		a, u := devirtualizeString(s)
		if u == nil {
			return p.regexpWrapper.findAllSubmatchIndex(string(a), limit, sticky)
		}
		if limit == 1 {
			result := p.regexpWrapper.findSubmatchIndexUnicode(u, p.unicode)
			if result.indexes == nil {
				return nil
			}
			return []regexpResult{result}
		}
		// Unfortunately Go's regexp library lacks FindAllReaderSubmatchIndex(), so we have to use a UTF-8 string as an
		// input.
		if p.unicode {
			// Try to convert s to UTF-8. If it does not contain any invalid UTF-16 we can do the matching in UTF-8.
			pm, str := buildUTF8PosMap(u)
			if pm != nil {
				res := p.regexpWrapper.findAllSubmatchIndex(str, limit, sticky)
				for _, result := range res {
					for i, idx := range result.indexes {
						result.indexes[i] = pm.get(idx)
					}
				}
				return res
			}
		}
	}

	p.createBacktracking()
	return p.backtrackingWrapper.findAllSubmatchIndex(s, start, limit, sticky, p.unicode)
}

// clone creates a copy of the regexpPattern which can be used concurrently.
func (p *regexpPattern) clone() *regexpPattern {
	ret := &regexpPattern{
		src:        p.src,
		global:     p.global,
		ignoreCase: p.ignoreCase,
		multiline:  p.multiline,
		dotAll:     p.dotAll,
		sticky:     p.sticky,
		unicode:    p.unicode,
	}
	if p.regexpWrapper != nil {
		ret.regexpWrapper = p.regexpWrapper.clone()
	}
	if p.backtrackingWrapper != nil {
		ret.backtrackingWrapper = p.backtrackingWrapper.clone()
	}
	return ret
}

type regexpObject struct {
	baseObject
	pattern *regexpPattern
	source  String

	standard bool
}

func (r *backtrackingWrapper) findSubmatchIndex(s String, start int, fullUnicode, doCache bool) regexpResult {
	if fullUnicode {
		return r.findSubmatchIndexUnicode(s, start, doCache)
	}
	return r.findSubmatchIndexUTF16(s, start, doCache)
}

func (r *backtrackingWrapper) findUTF16Cached(s String, start int, doCache bool) (match regexpBackendMatch, runes []rune, err error) {
	wrapped := r.rx
	cache := r.cache
	if cache != nil && cache.posMap == nil && cache.target.SameAs(s) {
		runes = cache.runes
	} else {
		runes = s.utf16Runes()
		cache = nil
	}
	match, err = wrapped.FindRunesMatchStartingAt(runes, start)
	if doCache && match != nil && err == nil {
		if cache == nil {
			if r.cache == nil {
				r.cache = new(backtrackingMatchCache)
			}
			*r.cache = backtrackingMatchCache{
				target: s,
				runes:  runes,
			}
		}
	} else {
		r.cache = nil
	}
	return
}

func (r *backtrackingWrapper) findSubmatchIndexUTF16(s String, start int, doCache bool) regexpResult {
	match, _, err := r.findUTF16Cached(s, start, doCache)
	if err != nil {
		return regexpResult{}
	}

	if match == nil {
		return regexpResult{}
	}
	groups := match.Groups()

	result := regexpResult{
		indexes: make([]int, 0, len(groups)<<1),
		groups:  make([]string, 0, len(groups)),
	}
	for _, group := range groups {
		if group.matched {
			result.appendIndexesAndGroup(group.index, group.index+group.length, group.name)
		} else {
			result.appendIndexesAndGroup(-1, 0, group.name)
		}
	}
	return result
}

func (r *backtrackingWrapper) findUnicodeCached(s String, start int, doCache bool) (match regexpBackendMatch, posMap []int, err error) {
	var (
		runes       []rune
		mappedStart int
		splitPair   bool
		savedRune   rune
	)
	wrapped := r.rx
	cache := r.cache
	if cache != nil && cache.posMap != nil && cache.target.SameAs(s) {
		runes, posMap = cache.runes, cache.posMap
		mappedStart, splitPair = posMapReverseLookup(posMap, start)
	} else {
		posMap, runes, mappedStart, splitPair = buildPosMap(&lenientUtf16Decoder{utf16Reader: s.utf16Reader()}, s.Length(), start)
		cache = nil
	}
	if splitPair {
		// temporarily set the rune at mappedStart to the second code point of the pair
		_, second := utf16.EncodeRune(runes[mappedStart])
		savedRune, runes[mappedStart] = runes[mappedStart], second
	}
	match, err = wrapped.FindRunesMatchStartingAt(runes, mappedStart)
	if doCache && match != nil && err == nil {
		if splitPair {
			runes[mappedStart] = savedRune
		}
		if cache == nil {
			if r.cache == nil {
				r.cache = new(backtrackingMatchCache)
			}
			*r.cache = backtrackingMatchCache{
				target: s,
				runes:  runes,
				posMap: posMap,
			}
		}
	} else {
		r.cache = nil
	}

	return
}

func (r *backtrackingWrapper) findSubmatchIndexUnicode(s String, start int, doCache bool) regexpResult {
	match, posMap, err := r.findUnicodeCached(s, start, doCache)
	if match == nil || err != nil {
		return regexpResult{}
	}

	groups := match.Groups()

	result := regexpResult{
		indexes: make([]int, 0, len(groups)<<1),
		groups:  make([]string, 0, len(groups)),
	}
	for _, group := range groups {
		if group.matched {
			result.appendIndexesAndGroup(posMap[group.index], posMap[group.index+group.length], group.name)
		} else {
			result.appendIndexesAndGroup(-1, 0, group.name)
		}
	}
	return result
}

func (r *backtrackingWrapper) findAllSubmatchIndexUTF16(s String, start, limit int, sticky bool) []regexpResult {
	wrapped := r.rx
	match, runes, err := r.findUTF16Cached(s, start, false)
	if match == nil || err != nil {
		return nil
	}
	if limit < 0 {
		limit = len(runes) + 1
	}
	results := make([]regexpResult, 0, limit)
	for match != nil {
		groups := match.Groups()

		result := regexpResult{
			indexes: make([]int, 0, len(groups)<<1),
			groups:  make([]string, 0, len(groups)),
		}

		for _, group := range groups {
			if group.matched {
				startPos := group.index
				endPos := group.index + group.length
				result.appendIndexesAndGroup(startPos, endPos, group.name)
			} else {
				result.appendIndexesAndGroup(-1, 0, group.name)
			}
		}

		if sticky && len(result.indexes) > 1 {
			if result.indexes[0] != start {
				break
			}
			start = result.indexes[1]
		}

		results = append(results, result)
		limit--
		if limit <= 0 {
			break
		}
		match, err = wrapped.FindNextMatch(match)
		if err != nil {
			return nil
		}
	}
	return results
}

func buildPosMap(rd io.RuneReader, l, start int) (posMap []int, runes []rune, mappedStart int, splitPair bool) {
	posMap = make([]int, 0, l+1)
	curPos := 0
	runes = make([]rune, 0, l)
	startFound := false
	for {
		if !startFound {
			if curPos == start {
				mappedStart = len(runes)
				startFound = true
			}
			if curPos > start {
				// start position splits a surrogate pair
				mappedStart = len(runes) - 1
				splitPair = true
				startFound = true
			}
		}
		rn, size, err := rd.ReadRune()
		if err != nil {
			break
		}
		runes = append(runes, rn)
		posMap = append(posMap, curPos)
		curPos += size
	}
	posMap = append(posMap, curPos)
	return
}

func posMapReverseLookup(posMap []int, pos int) (int, bool) {
	mapped := sort.SearchInts(posMap, pos)
	if mapped < len(posMap) && posMap[mapped] != pos {
		return mapped - 1, true
	}
	return mapped, false
}

func (r *backtrackingWrapper) findAllSubmatchIndexUnicode(s unicodeString, start, limit int, sticky bool) []regexpResult {
	wrapped := r.rx
	if limit < 0 {
		limit = len(s) + 1
	}
	results := make([]regexpResult, 0, limit)
	match, posMap, err := r.findUnicodeCached(s, start, false)
	if err != nil {
		return nil
	}
	for match != nil {
		groups := match.Groups()

		result := regexpResult{
			indexes: make([]int, 0, len(groups)<<1),
			groups:  make([]string, 0, len(groups)),
		}

		for _, group := range groups {
			if group.matched {
				start := posMap[group.index]
				end := posMap[group.index+group.length]
				result.appendIndexesAndGroup(start, end, group.name)
			} else {
				result.appendIndexesAndGroup(-1, 0, group.name)
			}
		}

		if sticky && len(result.indexes) > 1 {
			if result.indexes[0] != start {
				break
			}
			start = result.indexes[1]
		}

		results = append(results, result)
		match, err = wrapped.FindNextMatch(match)
		if err != nil {
			return nil
		}
	}
	return results
}

func (r *backtrackingWrapper) findAllSubmatchIndex(s String, start, limit int, sticky, fullUnicode bool) []regexpResult {
	a, u := devirtualizeString(s)
	if u != nil {
		if fullUnicode {
			return r.findAllSubmatchIndexUnicode(u, start, limit, sticky)
		}
		return r.findAllSubmatchIndexUTF16(u, start, limit, sticky)
	}
	return r.findAllSubmatchIndexUTF16(a, start, limit, sticky)
}

func (r *backtrackingWrapper) clone() *backtrackingWrapper {
	return &backtrackingWrapper{
		rx: r.rx,
	}
}

func (r *regexpWrapper) findAllSubmatchIndex(s string, limit int, sticky bool) []regexpResult {
	wrapped := (*regexp.Regexp)(r)
	results := submatchesToRegexpResults(wrapped.FindAllStringSubmatchIndex(s, limit), wrapped.SubexpNames())
	pos := 0
	if sticky {
		for i, result := range results {
			if len(result.indexes) > 1 {
				if result.indexes[0] != pos {
					return results[:i]
				}
				pos = result.indexes[1]
			}
		}
	}
	return results
}

func (r *regexpWrapper) findSubmatchIndex(s String, fullUnicode bool) regexpResult {
	a, u := devirtualizeString(s)
	if u != nil {
		return r.findSubmatchIndexUnicode(u, fullUnicode)
	}
	return r.findSubmatchIndexASCII(string(a))
}

func (r *regexpWrapper) findSubmatchIndexASCII(s string) regexpResult {
	wrapped := (*regexp.Regexp)(r)
	return regexpResult{indexes: wrapped.FindStringSubmatchIndex(s), groups: wrapped.SubexpNames()}
}

func (r *regexpWrapper) findSubmatchIndexUnicode(s unicodeString, fullUnicode bool) regexpResult {
	wrapped := (*regexp.Regexp)(r)
	if fullUnicode {
		posMap, runes, _, _ := buildPosMap(&lenientUtf16Decoder{utf16Reader: s.utf16Reader()}, s.Length(), 0)
		result := regexpResult{indexes: wrapped.FindReaderSubmatchIndex(&arrayRuneReader{runes: runes})}
		for i, item := range result.indexes {
			if item >= 0 {
				result.indexes[i] = posMap[item]
			}
		}
		return result
	}
	return regexpResult{indexes: wrapped.FindReaderSubmatchIndex(s.utf16RuneReader()), groups: wrapped.SubexpNames()}
}

func (r *regexpWrapper) clone() *regexpWrapper {
	return r
}

func (r *Runtime) createRegexpGroupsObj(valueArray []Value, groupNames []string) *Object {
	var groups *baseObject
	for index := 1; index < len(valueArray); index++ {
		if index < len(groupNames) {
			name := groupNames[index]
			if name == "" {
				continue
			} else if groups == nil {
				groups = r.newBaseObject(nil, classObject)
			}
			groups.setOwnStr(unistring.NewFromString(name), valueArray[index], false)
		}
	}
	if groups != nil {
		return groups.val
	}
	return nil
}

func createRegexpGroupsMap(result regexpResult) (groups map[unistring.String]int) {
	if len(result.groups) > 0 {
		groups = make(map[unistring.String]int)
		for i := 1; i < len(result.groups); i++ {
			if group := result.groups[i]; group != "" {
				idx := i * 2
				if idx < len(result.indexes)-1 {
					groups[unistring.NewFromString(group)] = idx
				}
			}
		}
	}
	return
}

func (r *regexpObject) execResultToArray(target String, result regexpResult) Value {
	captureCount := len(result.indexes) >> 1
	valueArray := make([]Value, captureCount)
	matchIndex := result.indexes[0]
	valueArray[0] = target.Substring(result.indexes[0], result.indexes[1])
	lowerBound := 0
	for index := 1; index < captureCount; index++ {
		offset := index << 1
		if result.indexes[offset] >= 0 && result.indexes[offset+1] >= lowerBound {
			valueArray[index] = target.Substring(result.indexes[offset], result.indexes[offset+1])
			lowerBound = result.indexes[offset]
		} else {
			valueArray[index] = _undefined
		}
	}
	match := r.val.runtime.newArrayValues(valueArray)
	match.self.setOwnStr("input", target, false)
	match.self.setOwnStr("index", intToValue(int64(matchIndex)), false)

	groups := r.val.runtime.createRegexpGroupsObj(valueArray, result.groups)

	var groupsVal Value
	if groups != nil {
		groupsVal = groups
	} else {
		groupsVal = _undefined
	}
	match.self.defineOwnPropertyStr("groups", PropertyDescriptor{
		Value:        groupsVal,
		Writable:     FLAG_TRUE,
		Configurable: FLAG_TRUE,
		Enumerable:   FLAG_TRUE,
	}, false)

	return match
}

func (r *regexpObject) getLastIndex() int64 {
	lastIndex := toLength(r.getStr("lastIndex", nil))
	if !r.pattern.global && !r.pattern.sticky {
		return 0
	}
	return lastIndex
}

func (r *regexpObject) execRegexp(target String) (match bool, result regexpResult) {
	index := r.getLastIndex()
	if index >= 0 && index <= int64(target.Length()) {
		result = r.pattern.findSubmatchIndex(target, int(index))
	}
	match = len(result.indexes) > 0 && (!r.pattern.sticky || int64(result.indexes[0]) == index)

	if r.pattern.global || r.pattern.sticky {
		var newLastIndex int64
		if match {
			newLastIndex = int64(result.indexes[1])
		}
		r.setOwnStr("lastIndex", intToValue(newLastIndex), true)
	}

	return
}

func (r *regexpObject) exec(target String) Value {
	match, result := r.execRegexp(target)
	if match {
		return r.execResultToArray(target, result)
	}
	return _null
}

func (r *regexpObject) test(target String) bool {
	match, _ := r.execRegexp(target)
	return match
}

func (r *regexpObject) clone() *regexpObject {
	r1 := r.val.runtime.newRegexpObject(r.prototype)
	r1.source = r.source
	r1.pattern = r.pattern

	return r1
}

func (r *regexpObject) init() {
	r.baseObject.init()
	r.standard = true
	r._putProp("lastIndex", intToValue(0), true, false, false)
}

func (r *regexpObject) setProto(proto *Object, throw bool) bool {
	res := r.baseObject.setProto(proto, throw)
	if res {
		r.standard = false
	}
	return res
}

func (r *regexpObject) defineOwnPropertyStr(name unistring.String, desc PropertyDescriptor, throw bool) bool {
	res := r.baseObject.defineOwnPropertyStr(name, desc, throw)
	if res {
		r.standard = false
	}
	return res
}

func (r *regexpObject) defineOwnPropertySym(name *Symbol, desc PropertyDescriptor, throw bool) bool {
	res := r.baseObject.defineOwnPropertySym(name, desc, throw)
	if res && r.standard {
		switch name {
		case SymMatch, SymMatchAll, SymSearch, SymSplit, SymReplace:
			r.standard = false
		}
	}
	return res
}

func (r *regexpObject) deleteStr(name unistring.String, throw bool) bool {
	res := r.baseObject.deleteStr(name, throw)
	if res {
		r.standard = false
	}
	return res
}

func (r *regexpObject) setOwnStr(name unistring.String, value Value, throw bool) bool {
	res := r.baseObject.setOwnStr(name, value, throw)
	if res && r.standard && name == "exec" {
		r.standard = false
	}
	return res
}

func (r *regexpObject) setOwnSym(name *Symbol, value Value, throw bool) bool {
	res := r.baseObject.setOwnSym(name, value, throw)
	if res && r.standard {
		switch name {
		case SymMatch, SymMatchAll, SymSearch, SymSplit, SymReplace:
			r.standard = false
		}
	}
	return res
}
