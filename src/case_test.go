package fzf

import (
	"slices"
	"testing"

	"github.com/junegunn/fzf/src/tui"
)

func TestChangeCaseSensitive(t *testing.T) {
	type step struct {
		arg   string
		mode  Case
		cycle []Case
	}
	star := []Case{CaseRespect, CaseSmart, CaseIgnore}
	for _, test := range []struct {
		name    string
		startup Case
		steps   []step
	}{
		{"default from ignore", CaseIgnore, []step{
			{"", CaseRespect, star}, {"", CaseSmart, star}, {"", CaseIgnore, star},
		}},
		{"default from smart", CaseSmart, []step{
			{"*", CaseIgnore, star}, {"", CaseRespect, star}, {"", CaseSmart, star},
		}},
		{"default from sensitive", CaseRespect, []step{
			{"*", CaseSmart, star}, {"", CaseIgnore, star}, {"", CaseRespect, star},
		}},
		{"replace shared cycle", CaseSmart, []step{
			{"ignore|no-ignore", CaseIgnore, []Case{CaseIgnore, CaseRespect}},
			{"", CaseRespect, []Case{CaseIgnore, CaseRespect}},
			{"", CaseIgnore, []Case{CaseIgnore, CaseRespect}},
			{"ignore", CaseIgnore, []Case{CaseIgnore}}, // Remember even without a mode change.
			{"", CaseIgnore, []Case{CaseIgnore}},
			{"no-ignore", CaseRespect, []Case{CaseRespect}},
			{"smart-case", CaseSmart, []Case{CaseSmart}},
			{"*", CaseIgnore, star}, {"", CaseRespect, star},
		}},
		{"nonadjacent duplicates", CaseIgnore, []step{
			{"ignore|smart-case|ignore|no-ignore|smart-case", CaseSmart, []Case{CaseIgnore, CaseSmart, CaseRespect}},
			{"", CaseRespect, []Case{CaseIgnore, CaseSmart, CaseRespect}},
			{"", CaseIgnore, []Case{CaseIgnore, CaseSmart, CaseRespect}},
		}},
		{"concretize ignore", CaseIgnore, []step{
			{"no-ignore|||ignore", CaseRespect, []Case{CaseRespect, CaseIgnore}},
			{"no-ignore||ignore", CaseIgnore, []Case{CaseRespect, CaseIgnore}},
			{"no-ignore", CaseRespect, []Case{CaseRespect}},
			{"|", CaseIgnore, []Case{CaseIgnore}}, {"", CaseIgnore, []Case{CaseIgnore}},
		}},
		{"concretize sensitive", CaseRespect, []step{
			{"|no-ignore", CaseRespect, []Case{CaseRespect}},
			{"no-ignore|||ignore", CaseIgnore, []Case{CaseRespect, CaseIgnore}},
			{"|", CaseRespect, []Case{CaseRespect}},
		}},
		{"concretize smart", CaseSmart, []step{
			{"no-ignore|||ignore", CaseIgnore, star},
			{"", CaseRespect, star},
			{"|no-ignore", CaseSmart, []Case{CaseSmart, CaseRespect}},
			{"|", CaseSmart, []Case{CaseSmart}},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			terminal := Terminal{caseMode: test.startup, caseModeDefault: test.startup}
			for _, step := range test.steps {
				before := terminal.caseMode
				changed, err := terminal.changeCaseSensitive(step.arg)
				if err != nil {
					t.Fatal(err)
				}
				if terminal.caseMode != step.mode || !slices.Equal(terminal.caseModeCycle, step.cycle) {
					t.Fatalf("%q: mode=%v cycle=%v, want mode=%v cycle=%v", step.arg,
						terminal.caseMode, terminal.caseModeCycle, step.mode, step.cycle)
				}
				if changed != (before != step.mode) || terminal.caseModeDefault != test.startup {
					t.Fatalf("%q: incorrect change flag or mutated startup mode", step.arg)
				}
			}
		})
	}
}

func TestChangeCaseSensitiveInvalid(t *testing.T) {
	for _, arg := range []string{"unknown", "ignore|unknown", "*|ignore", "ignore|*", " ignore", "IGNORE"} {
		terminal := Terminal{
			caseMode: CaseSmart, caseModeDefault: CaseIgnore,
			caseModeCycle: []Case{CaseSmart, CaseRespect},
		}
		if changed, err := terminal.changeCaseSensitive(arg); err == nil || changed {
			t.Fatalf("%q: expected error without mode change, got changed=%v err=%v", arg, changed, err)
		}
		if terminal.caseMode != CaseSmart || terminal.caseModeDefault != CaseIgnore ||
			!slices.Equal(terminal.caseModeCycle, []Case{CaseSmart, CaseRespect}) {
			t.Fatalf("%q: invalid action changed picker state", arg)
		}
		if _, err := parseSingleActionList("change-case-sensitive("+arg+")", false); err == nil {
			t.Fatalf("%q: action parser accepted invalid mode", arg)
		}
	}
}

func TestChangeCaseSensitiveBindings(t *testing.T) {
	keymap := defaultKeymap()
	err := parseKeymap(keymap, "ctrl-s:change-case-sensitive(),ctrl-r:change-case-sensitive(*)+up,"+
		"f1:change-case-sensitive(ignore|no-ignore),f2:change-case-sensitive(|),"+
		"f3:change-case-sensitive:smart-case")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		key tui.Event
		arg string
	}{
		{tui.CtrlS.AsEvent(), ""}, {tui.CtrlR.AsEvent(), "*"},
		{tui.F1.AsEvent(), "ignore|no-ignore"}, {tui.F2.AsEvent(), "|"}, {tui.F3.AsEvent(), "smart-case"},
	} {
		actions := keymap[test.key]
		if len(actions) == 0 || actions[0].t != actChangeCaseSensitive || actions[0].a != test.arg {
			t.Fatalf("%v: incorrect parsed case action", test.key)
		}
	}
	if actions := keymap[tui.CtrlR.AsEvent()]; len(actions) != 2 || actions[1].t != actUp {
		t.Fatal("case action lost following chained action")
	}
	// Different bindings share the last executed cycle, not their own cursor.
	terminal := Terminal{caseMode: CaseSmart, caseModeDefault: CaseSmart}
	for _, key := range []tui.Event{tui.F1.AsEvent(), tui.CtrlS.AsEvent(), tui.CtrlS.AsEvent()} {
		if _, err := terminal.changeCaseSensitive(keymap[key][0].a); err != nil {
			t.Fatal(err)
		}
	}
	if terminal.caseMode != CaseIgnore || keymap[tui.F1.AsEvent()][0].a != "ignore|no-ignore" {
		t.Fatal("cycle memory was not shared or action argument was rotated")
	}
}
