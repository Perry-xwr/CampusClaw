package app

import (
	"math"
	"testing"
)

func TestFuseRRF(t *testing.T) {
	keyword := []SearchHit{{ChunkID: 1}, {ChunkID: 2}}
	vector := []SearchHit{{ChunkID: 2}, {ChunkID: 3}}
	result := fuseRRF(keyword, vector, 10)
	if len(result) != 3 || result[0].ChunkID != 2 {
		t.Fatalf("unexpected order: %+v", result)
	}
	want := 1.0/61.0 + 1.0/62.0
	if math.Abs(result[0].Score-want) > 1e-12 {
		t.Fatalf("score=%v want=%v", result[0].Score, want)
	}
	if result[1].Score != 1.0/61.0 || result[1].KeywordRank != 1 || result[1].VectorRank != 0 {
		t.Fatalf("single path contribution wrong: %+v", result[1])
	}
}
