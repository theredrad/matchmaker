package matchmaker

import "context"

type Matchmaker interface {
	PushToQueue(ctx context.Context, id string, rank int, latency uint) error
	GetScore(ctx context.Context, id string, rank int, latency uint) (int64, error)
}
