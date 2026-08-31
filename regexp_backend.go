package goja

type regexpBackendOptions struct {
	multiline  bool
	dotAll     bool
	ignoreCase bool
	unicode    bool
}

type regexpBackend interface {
	FindRunesMatchStartingAt(input []rune, start int) (regexpBackendMatch, error)
	FindNextMatch(previous regexpBackendMatch) (regexpBackendMatch, error)
}

type regexpBackendMatch interface {
	Groups() []regexpBackendGroup
}

type regexpBackendGroup struct {
	index, length int
	name          string
	matched       bool
}
