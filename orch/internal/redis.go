package internal

import (
	"context"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
)

type redisSvc struct {
	rdb *redis.Client
}

func NewRedisSvc() *redisSvc {

	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST"),
		Password: os.Getenv("REDIS_PASS"),
		DB:       0,
	})

	r := &redisSvc{
		rdb: rdb,
	}

	//test
	go func() {
		Logger.Sugar().Debugf("[NewRedisSvc test]")
		go func() {
			r.PopAll("_testkey")
			Logger.Sugar().Debugf("[NewRedisSvc test], pushing _testkey in 3 sec")
			time.Sleep(3 * time.Second)
			err := r.LPush("_testkey", "foobar")
			if err != nil {
				Logger.Error("[NewRedisSvc test] failed to LPUSH _testkey: " + err.Error())
			}
		}()
		val, err := r.BLPop(10*time.Second, "_testkey")
		if err != nil {
			Logger.Sugar().Errorf("[NewRedisSvc test] failed -- err:%v", err)
		}
		Logger.Sugar().Debugf("[NewRedisSvc test] result: %v", val)
	}()
	return r
}

func (r *redisSvc) Client() *redis.Client {
	return r.rdb
}

func (r *redisSvc) PopAll(key string) {
	var err error
	for err == nil {
		_, err = r.rdb.LPop(context.Background(), key).Result()
	}
}

func (r *redisSvc) LPush(key, val string) error {
	Logger.Sugar().Debugf("LPush: %v:%v", key, val)
	_, err := r.rdb.LPush(context.Background(), key, val).Result()
	if err != nil {
		Logger.Sugar().Warnf("failed @ LPush (add retries?): %v", err)
	}
	return err
}
func (r *redisSvc) BLPop(timeout time.Duration, key string) ([]string, error) {
	val, err := r.rdb.BLPop(context.Background(), timeout, key).Result()
	if err != nil {
		Logger.Sugar().Warnf("failed BLPop (add retries?): %v", err)
	}
	return val, err
}

func (r *redisSvc) Get(key string) (string, error) {
	val, err := r.rdb.Get(context.Background(), key).Result()
	if err != nil {
		Logger.Sugar().Warnf("failed @ HSet (add retries?): %v", err)
	}
	return val, err
}

func (r *redisSvc) Set(key, val string) error {
	_, err := r.rdb.Set(context.Background(), key, val, 0).Result()
	if err != nil {
		Logger.Sugar().Warnf("failed @ HSet (add retries?): %v", err)
	}
	return err
}
