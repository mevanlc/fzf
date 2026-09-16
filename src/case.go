package fzf

import (
	"fmt"
	"slices"
	"strings"
)

// An empty list entry refers to the startup mode, which is not known when
// bindings are parsed: later command-line options can still override it.
const caseDefault Case = -1

func defaultCaseCycle() []Case {
	return []Case{CaseRespect, CaseSmart, CaseIgnore}
}

// A nil result means reuse the last executed cycle. Otherwise preserve all
// entries here; concretization must precede deduplication at execution time.
func parseCaseSensitiveModes(arg string) ([]Case, error) {
	switch arg {
	case "":
		return nil, nil
	case "*":
		return defaultCaseCycle(), nil
	}

	tokens := strings.Split(arg, "|")
	modes := make([]Case, 0, len(tokens))
	for _, token := range tokens {
		var mode Case
		switch token {
		case "":
			mode = caseDefault
		case "ignore":
			mode = CaseIgnore
		case "no-ignore":
			mode = CaseRespect
		case "smart-case":
			mode = CaseSmart
		default:
			return nil, fmt.Errorf("invalid change-case-sensitive mode: %q", token)
		}
		modes = append(modes, mode)
	}
	return modes, nil
}

// changeCaseSensitive updates picker-wide cycle memory even when the resulting
// mode is unchanged. The caller only needs to restart matching when it returns true.
func (t *Terminal) changeCaseSensitive(arg string) (bool, error) {
	modes, err := parseCaseSensitiveModes(arg)
	if err != nil {
		return false, err
	}
	cycle := t.caseModeCycle
	if modes != nil {
		cycle = make([]Case, 0, len(modes))
		for _, mode := range modes {
			if mode == caseDefault {
				mode = t.caseModeDefault
			}
			if !slices.Contains(cycle, mode) {
				cycle = append(cycle, mode)
			}
		}
	} else if len(cycle) == 0 {
		cycle = defaultCaseCycle()
	}

	next := cycle[(slices.Index(cycle, t.caseMode)+1)%len(cycle)]
	changed := next != t.caseMode
	t.caseModeCycle = cycle
	t.caseMode = next
	return changed, nil
}
