package elasticsearch

import (
	"context"
	"encoding/binary"
	"sort"
	"strings"

	"github.com/your-org/readonly-db-mcp/internal/config"
)

// An epsilon-NFA for a union of Elasticsearch '*'/'?' globs. Product traversal
// proves inclusion for future names as well as current index inventory. Literal
// characters plus one "other" symbol form a complete alphabet partition.
type globNFA struct {
	chars []rune
	ends  map[int]bool
	start []int
}

func compileGlobs(patterns []string) (globNFA, error) {
	n := globNFA{ends: map[int]bool{}}
	for _, pattern := range patterns {
		if !config.ValidElasticsearchPattern(pattern) {
			return n, failure("authority_unproven", "unsupported index privilege pattern syntax")
		}
		n.start = append(n.start, len(n.chars))
		n.chars = append(n.chars, []rune(pattern)...)
		n.ends[len(n.chars)] = true
		n.chars = append(n.chars, 0)
	}
	if len(n.chars) > 16384 {
		return n, failure("resource_limit", "index pattern proof exceeds state budget")
	}
	n.start = n.closure(n.start)
	return n, nil
}
func (n globNFA) closure(in []int) []int {
	seen := map[int]bool{}
	for _, p := range in {
		for {
			if seen[p] {
				break
			}
			seen[p] = true
			if n.chars[p] != '*' {
				break
			}
			p++
		}
	}
	out := make([]int, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}
func (n globNFA) step(state []int, c rune) []int {
	var next []int
	for _, p := range state {
		switch n.chars[p] {
		case '*':
			next = append(next, p)
		case '?':
			next = append(next, p+1)
		default:
			if n.chars[p] == c && c != 0 {
				next = append(next, p+1)
			}
		}
	}
	return n.closure(next)
}
func (n globNFA) accepts(state []int) bool {
	for _, p := range state {
		if n.ends[p] {
			return true
		}
	}
	return false
}
func stateKey(a, b []int) string {
	raw := make([]byte, 4*(len(a)+len(b)+1))
	i := 0
	for _, v := range a {
		binary.LittleEndian.PutUint32(raw[i:], uint32(v))
		i += 4
	}
	binary.LittleEndian.PutUint32(raw[i:], ^uint32(0))
	i += 4
	for _, v := range b {
		binary.LittleEndian.PutUint32(raw[i:], uint32(v))
		i += 4
	}
	return string(raw)
}
func globRelation(ctx context.Context, left, right []string, intersection bool) (bool, error) {
	a, err := compileGlobs(left)
	if err != nil {
		return false, err
	}
	b, err := compileGlobs(right)
	if err != nil {
		return false, err
	}
	chars := map[rune]bool{-1: true}
	for _, n := range []globNFA{a, b} {
		for _, r := range n.chars {
			if r != 0 && r != '*' && r != '?' {
				chars[r] = true
			}
		}
	}
	type pair struct{ a, b []int }
	queue := []pair{{a.start, b.start}}
	seen := map[string]bool{stateKey(a.start, b.start): true}
	transitions := 0
	retained := 0
	for head := 0; head < len(queue); head++ {
		if ctx.Err() != nil {
			return false, failure("deadline_exceeded", "index scope proof exceeded request deadline")
		}
		s := queue[head]
		acceptA, acceptB := a.accepts(s.a), b.accepts(s.b)
		if intersection && acceptA && acceptB {
			return true, nil
		}
		if !intersection && acceptA && !acceptB {
			return false, nil
		}
		for c := range chars {
			transitions++
			if transitions > 250000 {
				return false, failure("resource_limit", "index pattern proof exceeds transition budget")
			}
			x, y := a.step(s.a, c), b.step(s.b, c)
			if len(x) == 0 || (intersection && len(y) == 0) {
				continue
			}
			key := stateKey(x, y)
			if !seen[key] {
				retained += 16*(len(x)+len(y)) + len(key) + 128
				if retained > 1<<20 {
					return false, failure("resource_limit", "index pattern proof exceeds memory budget")
				}
				seen[key] = true
				queue = append(queue, pair{x, y})
				if len(queue) > 4096 {
					return false, failure("resource_limit", "index pattern proof exceeds state budget")
				}
			}
		}
	}
	return !intersection, nil
}

func scopePattern(ctx context.Context, pattern string, cfg *config.ElasticsearchConfig) error {
	contained, err := globRelation(ctx, []string{pattern}, cfg.AllowedIndices, false)
	if err != nil {
		return err
	}
	if !contained {
		return failure("scope_denied", "index expression exceeds configured scope")
	}
	denied := append(append([]string{}, cfg.DeniedIndices...), ".*")
	overlaps, err := globRelation(ctx, []string{pattern}, denied, true)
	if err != nil {
		return err
	}
	if overlaps {
		return failure("scope_denied", "index expression can reach denied or system indices")
	}
	return nil
}

func concreteName(name string) bool {
	return config.ValidElasticsearchPattern(name) && !strings.ContainsAny(name, "*?")
}
