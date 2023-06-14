package internal

import (
	"os"
	"time"

	"github.com/gomodule/redigo/redis"
)

type redisSvc struct {
	pool *redis.Pool
}

func NewRedisSvc() *redisSvc {

	const maxConnections = 10
	redisPool := &redis.Pool{
		MaxIdle: maxConnections,
		Dial: func() (redis.Conn, error) {
			return redis.Dial(
				"tcp",
				os.Getenv("REDIS_HOST"),
				redis.DialPassword(os.Getenv("REDIS_PASS")),
			)
		},
	}

	r := &redisSvc{
		pool: redisPool,
	}

	//test
	go func() {
		go func() {
			r.PopAll("_testkey")
			Logger.Sugar().Debugf("[NewRedisSvc test], pushing _testkey in 2 sec")
			time.Sleep(3 * time.Second)
			_, err := r.Conn().Do("LPUSH", "_testkey", "foobar")
			if err != nil {
				Logger.Error("[NewRedisSvc test] failed to LPUSH _testkey: " + err.Error())
			}
		}()
		val, err := r.Conn().Do("BLPop", "_testkey")
		if err != nil {
			Logger.Sugar().Errorf("[NewRedisSvc test] failed -- err:%v", err)
		}
		Logger.Sugar().Debugf("[NewRedisSvc test] result: %v", val)
	}()
	return r
}

func (r *redisSvc) Conn() redis.Conn {
	return r.pool.Get()
}

func (r *redisSvc) PopAll(key string) {
	var err error
	for err == nil {
		_, err = r.Conn().Do("LPOP", key)
	}
}

// func (r *redisSvc) Push(key, val string) error {
// 	_, err := r.Conn().Do("LPUSH", key, val)
// 	return err
// }
// func (r *redisSvc) Pop(key string) (interface{}, error) {
// 	val, err := r.Conn().Do("LPOP", key)
// 	return val, err
// }
