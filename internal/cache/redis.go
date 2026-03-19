package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/AVGsync/study_flow_api/internal/models"
	"github.com/redis/go-redis/v9"
)

type UserCache struct {
	client *redis.Client
	ttl 	 time.Duration
}

func NewUserCache(addr string, ttl time.Duration) *UserCache {
	return &UserCache{
		client: redis.NewClient(&redis.Options{
			Addr: addr,
		}),
		ttl: ttl,
	}
}

func (c *UserCache) SetUser(ctx context.Context, user *models.UserResponse) error {
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, "user:"+user.ID, data, c.ttl).Err()
}

func (c *UserCache) GetUser(ctx context.Context, id string) (*models.UserResponse, error) {
	data, err := c.client.Get(ctx, "user:"+id).Bytes()
	if err != nil {
		return nil, err
	}

	var user models.UserResponse
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, err
	}
	return &user, nil
}