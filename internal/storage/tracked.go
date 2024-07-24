package storage

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

type TrackedAmmStorage struct {
	client *redis.Client
}

const (
	TRACKED     = "TRACKED"
	PAUSE       = "PAUSE"
	NOT_TRACKED = "NOT_TRACKED"
)

func SetTracked(client *redis.Client, ammId string, status string) error {
	ctx := context.Background()

	if status != TRACKED && status != PAUSE && status != NOT_TRACKED {
		return errors.New("invalid tracking status")
	}

	if err := client.HSet(ctx, ammId, KEY_TRACKEDAMM, status).Err(); err != nil {
		return err
	}

	return nil
}

func GetTracked(client *redis.Client, ammId string) (string, error) {
	ctx := context.Background()
	status, err := client.HGet(ctx, ammId, KEY_TRACKEDAMM).Result()
	if err != nil {
		if err == redis.Nil {
			return NOT_TRACKED, nil
		}
		return "", err
	}

	switch status {
	case TRACKED, PAUSE, NOT_TRACKED:
		return status, nil
	default:
		return "", errors.New("unexpected value in Redis")
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
