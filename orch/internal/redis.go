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
	Logger.Sugar().Debugf("[NewRedisSvc test]")
	go func() {
		r.PopAll("_testkey")
		Logger.Sugar().Debugf("[NewRedisSvc test], pushing _testkey in 3 sec")
		time.Sleep(3 * time.Second)
		_, err := r.Conn().Do("LPUSH", "_testkey", "foobar")
		if err != nil {
			Logger.Error("[NewRedisSvc test] failed to LPUSH _testkey: " + err.Error())
		}
	}()
	val, err := r.Conn().Do("BLPop", "_testkey", 0)
	if err != nil {
		Logger.Sugar().Errorf("[NewRedisSvc test] failed -- err:%v", err)
	}
	Logger.Sugar().Debugf("[NewRedisSvc test] result: %v", val)

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

func (r *redisSvc) LPush(key string, val interface{}) error {
	_, err := r.Conn().Do("LPUSH", key, val)
	return err
}
func (r *redisSvc) BLPop(key string, sec int) (interface{}, error) {
	val, err := r.Conn().Do("BLPOP", key, sec)
	return val, err
}
