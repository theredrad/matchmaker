package redis

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
)

const (
	// default prefix for redis key
	defaultPrefix string = "matchmaker"
	// default max player
	defaultMaxPlayer int64 = 2

	// list key string format
	queueRankLatencyKeyFmt string = "%s:queue:rank_%d:latency_%d"
	// lock key string format
	queueMutexLockKeyFmt string = "%s:queue:rank_%d:latency_%d:match_lock"
)

// error types
var (
	ErrPlayerNotFoundInQueue = errors.New("player not found in queue")
)

// HandlerFunc is called when players matched
type HandlerFunc func(rank int, latency uint, IDs ...string)

type Player struct {
	ID      string
	Rank    int
	Latency uint
}

// Matchmaking options
type Options struct {
	// queue prefix
	Prefix string

	// Handler function to call when some players are matched
	Handler HandlerFunc

	// Matchmaker Logger
	Logger *log.Logger

	// MaxPlayer size for each match
	MaxPlayer int64
}

// Redis Matchmaker manage the queue, push players to the queue, match the players with same rank
type Matchmaker struct {
	// Redis client
	client *redis.Client

	// Redis lock to lock the list when pop from it
	locker *redsync.Redsync

	// logger instance
	logger *log.Logger

	// Matchmaker options
	opts *Options

	// a map of reset time for each queue to scoring players with joining time
	resetTimes map[string]time.Time
}

// NewMatchmaker accepts Redis client & matchmaking options & returns an instance of Redis Matchmaker
func NewMatchmaker(client *redis.Client, opts *Options) (*Matchmaker, error) {
	if opts == nil { // set default options if opts is nil
		opts = &Options{
			MaxPlayer: defaultMaxPlayer,
			Prefix:    defaultPrefix,
		}
	}

	if opts.Logger == nil {
		opts.Logger = log.New(os.Stderr, fmt.Sprintf("%s: ", opts.Prefix), log.LstdFlags|log.Lshortfile)
	}

	if opts.MaxPlayer <= 0 {
		opts.MaxPlayer = defaultMaxPlayer // set default max player for zero or negative numbers
	}

	matchmaker := &Matchmaker{
		opts:       opts,
		resetTimes: make(map[string]time.Time),
	}
	matchmaker.client = client
	pool := goredis.NewPool(matchmaker.client)
	matchmaker.locker = redsync.New(pool)
	return matchmaker, nil
}

// GetScore returns player queue score with the id, rank & latency
func (m *Matchmaker) GetScore(ctx context.Context, id string, rank int, latency uint) (int64, error) {
	res := m.client.ZRank(ctx, fmt.Sprintf(queueRankLatencyKeyFmt, m.opts.Prefix, rank, latency), id)
	score, err := res.Result()
	if err != nil {
		if err == redis.Nil {
			return 0, ErrPlayerNotFoundInQueue
		} else {
			return 0, err
		}
	}
	return score, nil
}

// PushToQueue pushes a player to the queue
func (m *Matchmaker) PushToQueue(ctx context.Context, id string, rank int, latency uint) error {
	return m.pushPlayerToQueue(ctx, &Player{
		id,
		rank,
		latency,
	})
}

func (m *Matchmaker) pushPlayerToQueue(ctx context.Context, player *Player) error {
	score := float64(time.Now().UnixNano())
	res := m.client.ZAdd(ctx, fmt.Sprintf(queueRankLatencyKeyFmt, m.opts.Prefix, player.Rank, player.Latency), &redis.Z{
		score,
		player.ID,
	})
	_, err := res.Result()
	if err != nil {
		return err
	}
	go m.match(ctx, player.Rank, player.Latency)
	return nil
}

// match locks the queue, query the queue size, if the size is greater than MaxPlayer, pop the players & calls the HandlerFunc, then release the lock
func (m *Matchmaker) match(ctx context.Context, rank int, latency uint) {
	mutex := m.locker.NewMutex(fmt.Sprintf(queueMutexLockKeyFmt, m.opts.Prefix, rank, latency))
	if err := mutex.Lock(); err != nil {
		m.opts.Logger.Printf("error while obtaining match function lock: %s", err.Error())
	}

	qLen := m.client.ZCard(ctx, fmt.Sprintf(queueRankLatencyKeyFmt, m.opts.Prefix, rank, latency))
	if qLen.Val() >= m.opts.MaxPlayer {
		result := m.client.ZPopMin(ctx, fmt.Sprintf(queueRankLatencyKeyFmt, m.opts.Prefix, rank, latency), m.opts.MaxPlayer)
		players := result.Val()

		var IDs []string
		for _, p := range players {
			IDs = append(IDs, fmt.Sprint(p.Member))
		}
		if m.opts.Handler != nil {
			go m.opts.Handler(rank, latency, IDs...)
		}

		if qLen.Val()-m.opts.MaxPlayer <= 0 { // queue got empty, so delete the reset time for the queue
			delete(m.resetTimes, fmt.Sprintf(queueRankLatencyKeyFmt, m.opts.Prefix, rank, latency)) //delete reset time for rank & latency
		}
	}

	ok, err := mutex.Unlock() // release the queue lock
	if err != nil {
		m.opts.Logger.Printf("error while releasing match function lock: %s", err.Error())
	}

	if !ok {
		m.opts.Logger.Printf("error while releasing match function lock: %s", "redis eval func returned 0 while releasing")
	}
}
