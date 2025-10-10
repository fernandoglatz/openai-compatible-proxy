package utils

import (
	"context"
	"encoding/json"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config"
	"time"

	"github.com/redis/go-redis/v9"
)

var RedisDatabase redisDatabaseType

type redisDatabaseType struct {
	Client *redis.Client
}

func ConnectToRedis(ctx context.Context) error {
	log.Info(ctx).Msg("Connecting to Redis")
	redisConfig := config.ApplicationConfig.Data.Redis

	redisOptions := &redis.Options{
		Addr:     redisConfig.Address,
		Password: redisConfig.Password,
		DB:       redisConfig.Db,
	}
	client := redis.NewClient(redisOptions)

	RedisDatabase = redisDatabaseType{
		Client: client,
	}

	cmd := client.Conn().Ping(ctx)
	err := cmd.Err()

	if err == nil {
		log.Info(ctx).Msg("Redis connected")
	} else {
		log.Error(ctx).Msg("Redis not connected: " + err.Error())
	}

	return nil
}

func (redisDatabase *redisDatabaseType) Get(ctx context.Context, key string) *redis.StringCmd {
	return redisDatabase.Client.Get(ctx, key)
}

func (redisDatabase *redisDatabaseType) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return redisDatabase.Client.Set(ctx, key, value, expiration).Err()
}

func (redisDatabase *redisDatabaseType) Del(ctx context.Context, key string) error {
	return redisDatabase.Client.Del(ctx, key).Err()
}

func (redisDatabase *redisDatabaseType) GetStruct(ctx context.Context, key string, value interface{}) error {
	cmd := redisDatabase.Get(ctx, key)

	err := cmd.Err()
	if err != nil {
		return err
	}

	jsonData, _ := cmd.Bytes()

	err = json.Unmarshal(jsonData, value)
	if err != nil {
		return err
	}

	return nil
}

func (redisDatabase *redisDatabaseType) SetStruct(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	jsonData, err := json.Marshal(value)
	if err != nil {
		return err
	}
	json := string(jsonData)

	return redisDatabase.Set(ctx, key, json, expiration)
}
