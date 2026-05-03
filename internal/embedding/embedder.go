package embedding

import "context"

type Embedder interface {
	EmbedStrings(ctx context.Context, texts []string) ([][]float64, error)
	Dimension() int
}
