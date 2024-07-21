package storage

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

type TrackedAmmStorage struct {
	client *redis.Client
}

func SetTracked(client *redis.Client, ammId string, tracked bool) error {
	ctx := context.Background()
	status := "NO"
	if tracked {
		status = "YES"
	}

	if err := client.HSet(ctx, ammId, KEY_TRACKEDAMM, status).Err(); err != nil {
		return err
	}

	return nil
}

func GetTracked(client *redis.Client, ammId string) (bool, error) {
	ctx := context.Background()
	isTracked, err := client.HGet(ctx, ammId, KEY_TRACKEDAMM).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}

	switch isTracked {
	case "YES":
		return true, nil
	case "NO":
		return false, nil
	default:
		return false, errors.New("unexpected value in Redis")
	}
}

func GetAllTracked(client *redis.Client) ([]string, error) {
	ctx := context.Background()
	keys, err := client.HKeys(ctx, KEY_TRACKEDAMM).Result()
	if err != nil {
		return nil, err
	}
	return keys, nil
}
