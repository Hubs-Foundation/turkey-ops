package internal

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisSvc struct {
	rdb *redis.Client
}

func NewRedisSvc() *redisSvc {
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST"),
		Password: os.Getenv("REDIS_PASS"),
		DB:       0, // use default DB
	})

	r := &redisSvc{
		rdb: rdb,
	}
	//test
	go func() {
		Logger.Sugar().Debugf("[NewRedisSvc test]")
		go func() {
			r.PopAll("_testkey")
			Logger.Sugar().Debugf("[NewRedisSvc test], pushing _testkey:foobar in 5 sec")
			time.Sleep(5 * time.Second)
			_, err := r.rdb.LPush(context.Background(), "_testkey", "foobar").Result()
			if err != nil {
				Logger.Error("[NewRedisSvc test] failed to LPUSH _testkey: " + err.Error())
			}
		}()
		val, err := r.rdb.BLPop(context.Background(), 10*time.Second, "_testkey").Result()
		if err != nil {
			Logger.Sugar().Errorf("[NewRedisSvc test] failed -- err:%v", err)
		}
		Logger.Sugar().Debugf("[NewRedisSvc test] result: %v", val)
	}()

	return r
}

func (r *redisSvc) PopAll(key string) {
	for val, err := r.rdb.LPop(context.Background(), key).Result(); err == nil; {
		Logger.Sugar().Debugf("dumped: %v", val)
	}
}

// type redisSvc struct {
// 	pool *redis.Pool
// }

// func NewRedisSvc() *redisSvc {

// 	const maxConnections = 10
// 	redisPool := &redis.Pool{
// 		MaxIdle: maxConnections,
// 		Dial: func() (redis.Conn, error) {
// 			return redis.Dial(
// 				"tcp",
// 				os.Getenv("REDIS_HOST"),
// 				redis.DialPassword(os.Getenv("REDIS_PASS")),
// 			)
// 		},
// 	}

// 	r := &redisSvc{
// 		pool: redisPool,
// 	}

// 	//test
// 	go func() {
// 		Logger.Sugar().Debugf("[NewRedisSvc test]")
// 		go func() {
// 			r.PopAll("_testkey")
// 			Logger.Sugar().Debugf("[NewRedisSvc test], pushing _testkey in 2 sec")
// 			time.Sleep(3 * time.Second)
// 			_, err := r.Conn().Do("LPUSH", "_testkey", "foobar")
// 			if err != nil {
// 				Logger.Error("[NewRedisSvc test] failed to LPUSH _testkey: " + err.Error())
// 			}
// 		}()
// 		val, err := r.Conn().Do("BLPop", "_testkey", 0)
// 		if err != nil {
// 			Logger.Sugar().Errorf("[NewRedisSvc test] failed -- err:%v", err)
// 		}
// 		Logger.Sugar().Debugf("[NewRedisSvc test] result: %v", val)
// 	}()
// 	return r
// }

// func (r *redisSvc) Conn() redis.Conn {
// 	return r.pool.Get()
// }

// func (r *redisSvc) PopAll(key string) {
// 	var err error
// 	for err == nil {
// 		_, err = r.Conn().Do("LPOP", key)
// 	}
// }

// // func (r *redisSvc) Push(key, val string) error {
// // 	_, err := r.Conn().Do("LPUSH", key, val)
// // 	return err
// // }
// // func (r *redisSvc) Pop(key string) (interface{}, error) {
// // 	val, err := r.Conn().Do("LPOP", key)
// // 	return val, err
// // }
