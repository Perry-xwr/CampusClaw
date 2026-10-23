package app

import (
	"math"
	"testing"
)

func TestFuseRRF(t *testing.T) {
	// The two channels intentionally have different Top-1 candidates. RRF must
	// rank the overlapping second keyword result first from its two contributions.
	keyword := []SearchHit{{ChunkID: 10}, {ChunkID: 20}}
	vector := []SearchHit{{ChunkID: 20}, {ChunkID: 30}}
	result := fuseRRF(keyword, vector, 10)
	if len(result) != 3 || result[0].ChunkID != 20 || result[1].ChunkID != 10 || result[2].ChunkID != 30 {
		t.Fatalf("unexpected order for different channel Top-1 candidates: %+v", result)
	}
	wants := []struct {
		chunkID int64
		score   float64
		kwRank  int
		vecRank int
	}{
		{20, 1.0/62.0 + 1.0/61.0, 2, 1},
		{10, 1.0 / 61.0, 1, 0},
		{30, 1.0 / 62.0, 0, 2},
	}
	seen := map[int64]bool{}
	for i, want := range wants {
		got := result[i]
		if seen[got.ChunkID] {
			t.Fatalf("duplicate fused chunk %d: %+v", got.ChunkID, result)
		}
		seen[got.ChunkID] = true
		if got.ChunkID != want.chunkID || math.Abs(got.Score-want.score) > 1e-12 || got.KeywordRank != want.kwRank || got.VectorRank != want.vecRank || got.Rank != i+1 {
			t.Fatalf("result[%d]=%+v want chunk=%d score=%v ranks=(%d,%d)", i, got, want.chunkID, want.score, want.kwRank, want.vecRank)
		}
	}
}

func TestFuseRRFTieBreakIsDeterministic(t *testing.T) {
	keyword := []SearchHit{{ChunkID: 20}, {ChunkID: 10}}
	vector := []SearchHit{{ChunkID: 10}, {ChunkID: 20}}
	for attempt := 0; attempt < 5; attempt++ {
		result := fuseRRF(keyword, vector, 10)
		if len(result) != 2 || result[0].ChunkID != 10 || result[1].ChunkID != 20 {
			t.Fatalf("tie-break was not deterministic by chunk ID: %+v", result)
		}
		if result[0].Score != result[1].Score || result[0].Rank != 1 || result[1].Rank != 2 {
			t.Fatalf("unexpected tied scores or fused ranks: %+v", result)
		}
	}
}
