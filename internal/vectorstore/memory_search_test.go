package vectorstore

import "testing"

func TestRankRecords(t *testing.T) {
	records := []VectorRecord{
		{ID: "low", DocumentID: "a", FileType: "markdown", Embedding: []float64{0, 1}},
		{ID: "high", DocumentID: "b", FileType: "markdown", Embedding: []float64{1, 0}},
	}
	results := RankRecords(records, []float64{1, 0}, SearchOptions{
		TopK:           1,
		ScoreThreshold: 0.1,
	})
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Record.ID != "high" {
		t.Fatalf("top result = %q, want high", results[0].Record.ID)
	}
}

func TestCosineSimilarityDimensionMismatch(t *testing.T) {
	if got := CosineSimilarity([]float64{1}, []float64{1, 2}); got != 0 {
		t.Fatalf("CosineSimilarity = %f, want 0", got)
	}
}
