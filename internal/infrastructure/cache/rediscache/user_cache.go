package rediscache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/AVGsync/study_flow_api/internal/model"
	"github.com/redis/go-redis/v9"
)

type UserCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewUserCache(addr string, ttl time.Duration) *UserCache {
	return &UserCache{
		client: redis.NewClient(&redis.Options{
			Addr: addr,
		}),
		ttl: ttl,
	}
}

func (c *UserCache) SetUser(ctx context.Context, user *model.UserResponse) error {
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, "user:"+user.ID, data, c.ttl).Err()
}

func (c *UserCache) GetUser(ctx context.Context, id string) (*model.UserResponse, error) {
	data, err := c.client.Get(ctx, "user:"+id).Bytes()
	if err != nil {
		return nil, err
	}

	var user model.UserResponse
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *UserCache) DeleteUser(ctx context.Context, id string) error {
	return c.client.Del(ctx, "user:"+id).Err()
}
