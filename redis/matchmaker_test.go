package redis

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestMatch(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer s.Close()

	c := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	ch := make(chan string)
	handler := func(rank int, latency uint, ids ...string) {
		go func() {
			ch <- strings.Join(ids, ",")
		}()
	}

	m, err := NewMatchmaker(c, &Options{
		Prefix:  "test",
		Handler: handler,
	})
	if err != nil {
		t.Errorf("expected new matchmaker, got err: %s", err)
		t.FailNow()
	}

	players := make(map[int]string)
	for i := 0; i < 10; i++ {
		players[i] = uuid.New().String()

		err = m.PushToQueue(context.Background(), players[i], 1, 30)
		if err != nil {
			t.Errorf("expected pushed queue, got err: %s", err)
			t.FailNow()
		}
		t.Logf("player %s addded to queue", players[i])
	}

	for i := 0; i < 5; i++ {
		p := <-ch
		if p != strings.Join([]string{players[i*2], players[i*2+1]}, ",") {
			t.Errorf("expected matched players, got wrong matched: %s", p)
			t.Fail()
		} else {
			t.Logf("players are matched: %s", p)
		}
	}
}

func TestMatchWithRank(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer s.Close()

	c := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	type rankedMatched struct {
		Rank  int
		Match string
	}
	ch := make(chan *rankedMatched)
	handler := func(rank int, latency uint, ids ...string) {
		go func() {
			ch <- &rankedMatched{
				Rank:  rank,
				Match: strings.Join(ids, ","),
			}
		}()
	}

	m, err := NewMatchmaker(c, &Options{
		Prefix:  "test",
		Handler: handler,
	})
	if err != nil {
		t.Errorf("expected new matchmaker, got err: %s", err)
		t.FailNow()
	}

	players := make(map[int]string)
	for i := 0; i < 12; i++ {
		players[i] = uuid.New().String()

		rank := 1
		if i%2 == 0 {
			rank = 2
		}
		err = m.PushToQueue(context.Background(), players[i], rank, 30)
		if err != nil {
			t.Errorf("expected pushed queue, got err: %s", err)
			t.FailNow()
		}
		t.Logf("player %s addded to queue with rank %d", players[i], rank)
	}

	e := 0
	o := 1
	var j int
	for i := 0; i < 6; i++ {
		p := <-ch

		if p.Rank == 2 {
			j = e
			e += 4
		} else {
			j = o
			o += 4
		}

		if p.Match != strings.Join([]string{players[j], players[j+2]}, ",") {
			t.Errorf("expected matched players, got wrong matched: %s", p.Match)
			t.Fail()
		} else {
			t.Logf("players are matched: %s", p.Match)
		}
	}
}
