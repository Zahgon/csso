package csstree

import (
	"strconv"
	"strings"
	"testing"
)

// mulberry32 is the seeded generator the recorder uses to script list
// mutations, so both sides draw the same sequence.
func mulberry32(seed uint32) func() float64 {
	a := seed
	return func() float64 {
		a += 0x6D2B79F5
		t := (a ^ a>>15) * (1 | a)
		t = (t + (t^t>>7)*(61|t)) ^ t
		return float64(t^t>>14) / 4294967296
	}
}

func identNames(l *List) string {
	var names []string
	for n := range l.All() {
		names = append(names, n.(*Identifier).Name)
	}
	return strings.Join(names, " ")
}

// TestListMutationDuringIteration replays random mutations performed from
// inside iteration callbacks and compares the visit order and the final
// list with css-tree's List.
func TestListMutationDuringIteration(t *testing.T) {
	type recordedList struct {
		Seed    uint32 `json:"seed"`
		Mode    string `json:"mode"`
		Visited string `json:"visited"`
		Final   string `json:"final"`
		Back    string `json:"back"`
	}
	var fx struct {
		Lists []recordedList `json:"lists"`
	}
	loadFixtureInto(t, &fx)
	if len(fx.Lists) == 0 {
		t.Fatal("no list fixtures")
	}

	for _, rec := range fx.Lists {
		rnd := mulberry32(rec.Seed)
		mk := func(name string) Node { return &Identifier{Name: name} }
		list := NewList()
		for i := 0; i < 1+int(rec.Seed%9); i++ {
			list.AppendData(mk("n" + strconv.Itoa(i)))
		}
		var visited []string
		counter, calls := 0, 0
		next := func() string {
			counter++
			return "x" + strconv.Itoa(counter-1)
		}
		cb := func(data Node, item *ListItem) bool {
			visited = append(visited, data.(*Identifier).Name)
			calls++
			if calls > 40 {
				return true
			}
			switch int(rnd() * 10) {
			case 0:
				list.Remove(item)
			case 1:
				if item.Next != nil {
					list.Remove(item.Next)
				}
			case 2:
				if item.Prev != nil {
					list.Remove(item.Prev)
				}
			case 3:
				list.InsertData(mk(next()), item)
			case 4:
				list.AppendData(mk(next()))
			case 5:
				list.PrependData(mk(next()))
			case 6:
				list.Replace(item, NewListItem(mk(next())))
			case 7:
				a, b := next(), next()
				list.InsertList(NewListOf(mk(a), mk(b)), item.Next)
			case 8:
				a, b := next(), next()
				list.ReplaceWithList(item, NewListOf(mk(a), mk(b)))
			}
			return rnd() < 0.1
		}

		switch rec.Mode {
		case "forEach":
			list.ForEach(func(data Node, item *ListItem) { cb(data, item) })
		case "forEachRight":
			list.ForEachRight(func(data Node, item *ListItem) { cb(data, item) })
		case "nextUntil":
			var start *ListItem
			if list.Head != nil {
				start = list.Head.Next
			}
			list.NextUntil(start, cb)
		case "prevUntil":
			list.PrevUntil(list.Tail, cb)
		case "reduce":
			Reduce(list, func(acc bool, data Node, item *ListItem) bool { return cb(data, item) || acc }, false)
		case "reduceRight":
			ReduceRight(list, func(acc bool, data Node, item *ListItem) bool { return cb(data, item) || acc }, false)
		}

		var back []string
		for it := list.Tail; it != nil; it = it.Prev {
			back = append(back, it.Data.(*Identifier).Name)
		}
		if got := strings.Join(visited, " "); got != rec.Visited {
			t.Errorf("seed %d %s: visited %q, want %q", rec.Seed, rec.Mode, got, rec.Visited)
		}
		if got := identNames(list); got != rec.Final {
			t.Errorf("seed %d %s: final %q, want %q", rec.Seed, rec.Mode, got, rec.Final)
		}
		if got := strings.Join(back, " "); got != rec.Back {
			t.Errorf("seed %d %s: backwards %q, want %q", rec.Seed, rec.Mode, got, rec.Back)
		}
	}
}
