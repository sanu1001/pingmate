package config

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

var Redis *redis.Client

func ConnectRedis() {
	rdb := redis.NewClient(&redis.Options{
		Addr: App.RedisAddr,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Redis ping failed — is redis running? %v", err)
	}

	Redis = rdb
	log.Println("✓ Redis connected")
}
