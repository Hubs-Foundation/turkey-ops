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

func (r *redisSvc) PopAll(key string) {
	var err error
	for err == nil {
		_, err = r.rdb.LPop(context.Background(), key).Result()
	}
}

func (r *redisSvc) LPush(key string, val interface{}) error {
	_, err := r.rdb.LPush(context.Background(), key, val).Result()
	Logger.Sugar().Errorf("failed (add retries?): %v", err)
	return err
}
func (r *redisSvc) BLPop(timeout time.Duration, key string) (interface{}, error) {
	val, err := r.rdb.BLPop(context.Background(), timeout, key).Result()
	Logger.Sugar().Errorf("failed (add retries?): %v", err)
	return val, err
}
