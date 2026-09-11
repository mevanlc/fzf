package fzf

import (
	"fmt"
	"testing"

	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
)

func TestMatcherCaseModeChanges(t *testing.T) {
	algo.Init("default")
	for _, extended := range []bool{false, true} {
		t.Run(fmt.Sprintf("extended=%v", extended), func(t *testing.T) {
			cache := NewChunkCache()
			var index int32
			list := NewChunkList(cache, func(item *Item, data []byte) bool {
				item.text = util.ToChars(data)
				item.text.Index = index
				index++
				return true
			})
			list.Push([]byte("INSTALL"))
			list.Push([]byte("install.sh"))
			// Full chunks exercise the bitmap cache as well as the merger cache.
			for range chunkSize*2 - 2 {
				list.Push([]byte("unrelated"))
			}
			chunks, _, _ := list.Snapshot(0)
			mode := CaseIgnore
			patterns := make(map[string]*Pattern)
			builder := func(query []rune) *Pattern {
				return BuildPattern(cache, patterns, true, algo.FuzzyMatchV2,
					extended, mode, true, true, false, true,
					nil, Delimiter{}, revision{}, query, nil, 0)
			}
			events := util.NewEventBox()
			matcher := NewMatcher(cache, builder, false, false, events, revision{}, 2)
			done := make(chan struct{})
			go func() {
				matcher.Loop()
				close(done)
			}()
			defer func() {
				matcher.Stop()
				<-done
			}()

			for cycle := range 3 {
				for _, current := range []Case{CaseIgnore, CaseSmart, CaseRespect} {
					mode = current
					patterns = make(map[string]*Pattern)
					for queryIndex, query := range []string{"inst", "INSTALL", "instALL", "inst"} {
						want := 2
						if mode == CaseRespect || mode == CaseSmart && query != "inst" {
							want = 1
							if query == "instALL" {
								want = 0
							}
						}
						matcher.Reset(chunks, []rune(query), true, true, false, revision{})
						events.WaitFor(EvtSearchFin)
						var got int
						events.Wait(func(pending *util.Events) {
							got = (*pending)[EvtSearchFin].(MatchResult).merger.Length()
							pending.Clear()
						})
						if got != want {
							t.Fatalf("cycle %d, %s, query %d %q: got %d matches, want %d", cycle, mode, queryIndex, query, got, want)
						}
					}
				}
			}
		})
	}
}
