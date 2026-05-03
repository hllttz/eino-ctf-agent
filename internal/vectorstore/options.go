package vectorstore

type SearchOptions struct {
	TopK           int
	ScoreThreshold float64
	DocumentIDs    []string
	FileTypes      []string
}
