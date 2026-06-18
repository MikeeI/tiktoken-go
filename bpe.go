package tiktoken

import "math"

const heapMergeMinBytes = 512

type mergePart struct {
	start      int
	rank       int
	prev       int
	next       int
	alive      bool
	generation int
}

type mergeCandidate struct {
	rank       int
	index      int
	generation int
}

type mergeCandidateHeap []mergeCandidate

func (h mergeCandidateHeap) Len() int {
	return len(h)
}

func (h mergeCandidateHeap) Less(i, j int) bool {
	if h[i].rank != h[j].rank {
		return h[i].rank < h[j].rank
	}
	return h[i].index < h[j].index
}

func (h mergeCandidateHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *mergeCandidateHeap) Push(candidate mergeCandidate) {
	*h = append(*h, candidate)
	for child := len(*h) - 1; child > 0; {
		parent := (child - 1) / 2
		if !h.Less(child, parent) {
			break
		}
		h.Swap(parent, child)
		child = parent
	}
}

func (h *mergeCandidateHeap) Pop() mergeCandidate {
	old := *h
	item := old[0]
	last := old[len(old)-1]
	old = old[:len(old)-1]
	if len(old) > 0 {
		old[0] = last
		*h = old
		for parent := 0; ; {
			left := parent*2 + 1
			if left >= len(*h) {
				break
			}
			child := left
			right := left + 1
			if right < len(*h) && h.Less(right, left) {
				child = right
			}
			if !h.Less(child, parent) {
				break
			}
			h.Swap(parent, child)
			parent = child
		}
		return item
	}
	*h = old
	return item
}

//nolint:gocognit // Byte-pair merge mirrors upstream logic; refactoring risks tokenizer parity.
func bytePairMerge[T any](piece string, ranks map[string]int, f func(start, end int) T) []T {
	if len(piece) < heapMergeMinBytes {
		return bytePairMergeScan(piece, ranks, f)
	}
	return bytePairMergeHeap(piece, ranks, f)
}

//nolint:gocognit // Byte-pair merge mirrors upstream logic; refactoring risks tokenizer parity.
func bytePairMergeScan[T any](piece string, ranks map[string]int, f func(start, end int) T) []T {
	parts := make([][2]int, len(piece)+1)
	for i := range parts {
		parts[i][0], parts[i][1] = i, math.MaxInt
	}

	getRank := func(startIdx, skip int) int {
		if startIdx+skip+2 < len(parts) {
			if rank, ok := ranks[piece[parts[startIdx][0]:parts[startIdx+skip+2][0]]]; ok {
				return rank
			}
		}
		return -1
	}

	for i := range len(parts) - 2 {
		if rank := getRank(i, 0); rank >= 0 {
			parts[i][1] = rank
		}
	}

	for len(parts) > 1 {
		minRank, minIdx := math.MaxInt, -1
		for i := range len(parts) - 1 {
			if parts[i][1] < minRank {
				minRank, minIdx = parts[i][1], i
			}
		}

		if minRank == math.MaxInt {
			break
		}

		i := minIdx
		if rank := getRank(i, 1); rank >= 0 {
			parts[i][1] = rank
		} else {
			parts[i][1] = math.MaxInt
		}
		if i > 0 {
			if rank := getRank(i-1, 1); rank >= 0 {
				parts[i-1][1] = rank
			} else {
				parts[i-1][1] = math.MaxInt
			}
		}
		parts = append(parts[:i+1], parts[i+2:]...)
	}

	out := make([]T, len(parts)-1)
	for i := range out {
		out[i] = f(parts[i][0], parts[i+1][0])
	}
	return out
}

//nolint:gocognit // Heap variant preserves scan tie-breaking for long pieces.
func bytePairMergeHeap[T any](piece string, ranks map[string]int, f func(start, end int) T) []T {
	parts := make([]mergePart, len(piece)+1)
	for i := range parts {
		parts[i] = mergePart{
			start: i,
			rank:  math.MaxInt,
			prev:  i - 1,
			next:  i + 1,
			alive: true,
		}
	}
	parts[0].prev = -1
	parts[len(parts)-1].next = -1

	getRank := func(index int) int {
		if index < 0 || !parts[index].alive {
			return math.MaxInt
		}
		next := parts[index].next
		if next < 0 || !parts[next].alive {
			return math.MaxInt
		}
		nextNext := parts[next].next
		if nextNext < 0 || !parts[nextNext].alive {
			return math.MaxInt
		}
		if rank, ok := ranks[piece[parts[index].start:parts[nextNext].start]]; ok {
			return rank
		}
		return math.MaxInt
	}

	pushRank := func(candidates *mergeCandidateHeap, index int) {
		parts[index].rank = getRank(index)
		if parts[index].rank == math.MaxInt {
			return
		}
		candidates.Push(mergeCandidate{
			rank:       parts[index].rank,
			index:      index,
			generation: parts[index].generation,
		})
	}

	candidates := make(mergeCandidateHeap, 0, len(piece))
	for i := range len(parts) - 1 {
		pushRank(&candidates, i)
	}

	for candidates.Len() > 0 {
		candidate := candidates.Pop()
		part := &parts[candidate.index]
		if !part.alive || candidate.generation != part.generation || part.next < 0 || part.rank != candidate.rank {
			continue
		}

		next := part.next
		following := parts[next].next
		if following < 0 {
			part.rank = math.MaxInt
			continue
		}

		parts[next].alive = false
		part.next = following
		parts[following].prev = candidate.index

		part.generation++
		pushRank(&candidates, candidate.index)

		if prev := part.prev; prev >= 0 {
			parts[prev].generation++
			pushRank(&candidates, prev)
		}
	}

	out := make([]T, 0, len(piece))
	for i := 0; i >= 0 && parts[i].next >= 0; i = parts[i].next {
		out = append(out, f(parts[i].start, parts[parts[i].next].start))
	}
	return out
}

func bytePairEncode(piece string, ranks map[string]int) []int {
	if len(piece) == 1 {
		v := ranks[piece]
		return []int{v}
	}
	return bytePairMerge(piece, ranks, func(start, end int) int {
		return ranks[piece[start:end]]
	})
}
