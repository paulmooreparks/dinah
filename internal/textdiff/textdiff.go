// Package textdiff compares sequences the way Python's difflib does: the
// Ratcliff/Obershelp matcher with its autojunk heuristic, which finds the
// longest matching runs and reports the differences around them. It works on
// sequences of integer tokens, so callers compare lines, blocks or anything
// else they can number, and it renders line diffs in the unified format.
package textdiff

import (
	"fmt"
	"sort"
	"strings"
)

// Opcode is one step from a to b: a[I1:I2] equals, replaces, is deleted from,
// or (for an insert, with I1 == I2) is followed by b[J1:J2].
type Opcode struct {
	Tag            byte // 'e' equal, 'r' replace, 'd' delete, 'i' insert
	I1, I2, J1, J2 int
}

// Tokens numbers the strings of each slice so that equal strings get equal
// numbers across both, ready for Opcodes.
func Tokens(a, b []string) ([]int, []int) {
	ids := map[string]int{}
	tok := func(items []string) []int {
		out := make([]int, len(items))
		for i, s := range items {
			id, ok := ids[s]
			if !ok {
				id = len(ids)
				ids[s] = id
			}
			out[i] = id
		}
		return out
	}
	return tok(a), tok(b)
}

// Lines splits text into lines that keep their line endings.
func Lines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.SplitAfter(s, "\n")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// UnifiedDiff renders the change from before to after as a unified diff with
// three lines of context, as difflib.unified_diff renders it.
func UnifiedDiff(before, after, fromFile, toFile string) string {
	a, b := Lines(before), Lines(after)
	ta, tb := Tokens(a, b)
	var out strings.Builder
	started := false
	for _, group := range Grouped(Opcodes(ta, tb), 3) {
		if !started {
			started = true
			fmt.Fprintf(&out, "--- %s\n+++ %s\n", fromFile, toFile)
		}
		first, last := group[0], group[len(group)-1]
		fmt.Fprintf(&out, "@@ -%s +%s @@\n", unifiedRange(first.I1, last.I2), unifiedRange(first.J1, last.J2))
		for _, op := range group {
			if op.Tag == 'e' {
				for _, l := range a[op.I1:op.I2] {
					out.WriteString(" " + l)
				}
				continue
			}
			if op.Tag == 'r' || op.Tag == 'd' {
				for _, l := range a[op.I1:op.I2] {
					out.WriteString("-" + l)
				}
			}
			if op.Tag == 'r' || op.Tag == 'i' {
				for _, l := range b[op.J1:op.J2] {
					out.WriteString("+" + l)
				}
			}
		}
	}
	return out.String()
}

func unifiedRange(start, stop int) string {
	beginning, length := start+1, stop-start
	if length == 1 {
		return fmt.Sprint(beginning)
	}
	if length == 0 {
		beginning--
	}
	return fmt.Sprintf("%d,%d", beginning, length)
}

type block struct{ i, j, k int }

func matchingBlocks(a, b []int) []block {
	b2j := map[int][]int{}
	for i, x := range b {
		b2j[x] = append(b2j[x], i)
	}
	if n := len(b); n >= 200 {
		ntest := n/100 + 1
		for x, idx := range b2j {
			if len(idx) > ntest {
				delete(b2j, x)
			}
		}
	}
	longest := func(alo, ahi, blo, bhi int) block {
		besti, bestj, bestk := alo, blo, 0
		j2len := map[int]int{}
		for i := alo; i < ahi; i++ {
			next := map[int]int{}
			for _, j := range b2j[a[i]] {
				if j < blo {
					continue
				}
				if j >= bhi {
					break
				}
				k := j2len[j-1] + 1
				next[j] = k
				if k > bestk {
					besti, bestj, bestk = i-k+1, j-k+1, k
				}
			}
			j2len = next
		}
		for besti > alo && bestj > blo && a[besti-1] == b[bestj-1] {
			besti, bestj, bestk = besti-1, bestj-1, bestk+1
		}
		for besti+bestk < ahi && bestj+bestk < bhi && a[besti+bestk] == b[bestj+bestk] {
			bestk++
		}
		return block{besti, bestj, bestk}
	}
	type span struct{ alo, ahi, blo, bhi int }
	queue := []span{{0, len(a), 0, len(b)}}
	var found []block
	for len(queue) > 0 {
		s := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		x := longest(s.alo, s.ahi, s.blo, s.bhi)
		if x.k > 0 {
			found = append(found, x)
			if s.alo < x.i && s.blo < x.j {
				queue = append(queue, span{s.alo, x.i, s.blo, x.j})
			}
			if x.i+x.k < s.ahi && x.j+x.k < s.bhi {
				queue = append(queue, span{x.i + x.k, s.ahi, x.j + x.k, s.bhi})
			}
		}
	}
	sort.Slice(found, func(p, q int) bool {
		if found[p].i != found[q].i {
			return found[p].i < found[q].i
		}
		return found[p].j < found[q].j
	})
	var out []block
	i1, j1, k1 := 0, 0, 0
	for _, x := range found {
		if i1+k1 == x.i && j1+k1 == x.j {
			k1 += x.k
		} else {
			if k1 > 0 {
				out = append(out, block{i1, j1, k1})
			}
			i1, j1, k1 = x.i, x.j, x.k
		}
	}
	if k1 > 0 {
		out = append(out, block{i1, j1, k1})
	}
	return append(out, block{len(a), len(b), 0})
}

// Opcodes returns the steps that turn a into b.
func Opcodes(a, b []int) []Opcode {
	var out []Opcode
	i, j := 0, 0
	for _, m := range matchingBlocks(a, b) {
		var tag byte
		switch {
		case i < m.i && j < m.j:
			tag = 'r'
		case i < m.i:
			tag = 'd'
		case j < m.j:
			tag = 'i'
		}
		if tag != 0 {
			out = append(out, Opcode{tag, i, m.i, j, m.j})
		}
		i, j = m.i+m.k, m.j+m.k
		if m.k > 0 {
			out = append(out, Opcode{'e', m.i, i, m.j, j})
		}
	}
	return out
}

// Grouped splits opcodes into hunks with n steps of unchanged context on each
// side, as difflib's get_grouped_opcodes does.
func Grouped(codes []Opcode, n int) [][]Opcode {
	if len(codes) == 0 {
		codes = []Opcode{{'e', 0, 1, 0, 1}}
	}
	if c := codes[0]; c.Tag == 'e' {
		codes[0] = Opcode{'e', max(c.I1, c.I2-n), c.I2, max(c.J1, c.J2-n), c.J2}
	}
	if c := codes[len(codes)-1]; c.Tag == 'e' {
		codes[len(codes)-1] = Opcode{'e', c.I1, min(c.I2, c.I1+n), c.J1, min(c.J2, c.J1+n)}
	}
	nn := n + n
	var groups [][]Opcode
	var group []Opcode
	for _, c := range codes {
		if c.Tag == 'e' && c.I2-c.I1 > nn {
			group = append(group, Opcode{'e', c.I1, min(c.I2, c.I1+n), c.J1, min(c.J2, c.J1+n)})
			groups = append(groups, group)
			group = nil
			c.I1, c.J1 = max(c.I1, c.I2-n), max(c.J1, c.J2-n)
		}
		group = append(group, c)
	}
	if len(group) > 0 && !(len(group) == 1 && group[0].Tag == 'e') {
		groups = append(groups, group)
	}
	return groups
}
