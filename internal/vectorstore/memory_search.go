package vectorstore

import (
	"math"
	"sort"
)

func CosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func RankRecords(records []VectorRecord, query []float64, opts SearchOptions) []SearchResult {
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	documentIDs := toSet(opts.DocumentIDs)
	fileTypes := toSet(opts.FileTypes)

	results := make([]SearchResult, 0, len(records))
	for _, record := range records {
		if len(documentIDs) > 0 && !documentIDs[record.DocumentID] {
			continue
		}
		if len(fileTypes) > 0 && !fileTypes[record.FileType] {
			continue
		}
		score := CosineSimilarity(query, record.Embedding)
		if score < opts.ScoreThreshold {
			continue
		}
		results = append(results, SearchResult{Record: record, Score: score})
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Record.ChunkIndex < results[j].Record.ChunkIndex
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results
}

func toSet(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]bool, len(items))
	for _, item := range items {
		if item != "" {
			set[item] = true
		}
	}
	return set
}
