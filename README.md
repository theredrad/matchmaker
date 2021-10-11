# Matchmaker
This package is a simple FIFO matchmaker that supports player rank & latency (as tags). The rank & latency tags help you to group the players.

## Implementations
### Redis
This implementation uses `Redis` for Queue & scoring with the mutex locking support. for now, the score is time, a lower score has higher priority (FIFO)

```go
import (
	"github.com/go-redis/redis/v8"
	mRedis "github.com/theredrad/matchmaker/redis"
)

func main() {
    client := redis.NewClient(&redis.Options{
        Addr:     "REDIS_ENDPOINT:PORT",
    })
    
    matchmaker, err := mRedis.NewMatchmaker(client, &mRedis.Options{
        Prefix:    "test_matchmaking",
        Handler:   MatchHandler, // handler func
        MaxPlayer: 2,
    })
    
    matchmaker.PushToQueue(context.Background(), "PLAYER_ID", 10, 100) // pass player rank as 10 & latency group as <= 100ms
    matchmaker.PushToQueue(context.Background(), "PLAYER_2_ID", 10, 100) // pass player rank as 10 & latency group as <= 100ms
    
    // wait for exit signal
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    <-c
}

func MatchHandler (rank int, latency uint, ids ...string) { 
	// list of matched players ID
}
```


## TODO
- [ ] Add timeout option to match players from different ranks or latency if no matched player is found in the same group

