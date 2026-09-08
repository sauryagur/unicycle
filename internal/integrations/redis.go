package integrations

import (
	"context"
	"github.com/redis/go-redis/v9"
	"os"
	"time"
)

type Redis struct{ Client *redis.Client }

func NewRedis() *Redis {
	return &Redis{Client: redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR"), Password: os.Getenv("REDIS_PASSWORD"), DB: 0})}
}
func (r *Redis) Ping(ctx context.Context) error { return r.Client.Ping(ctx).Err() }
func (r *Redis) Reserve(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return r.Client.SetNX(ctx, key, "1", ttl).Result()
}
func (r *Redis) Release(ctx context.Context, key string) error { return r.Client.Del(ctx, key).Err() }
func (r *Redis) Close() error                                  { return r.Client.Close() }
