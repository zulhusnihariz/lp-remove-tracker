package storage

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/iqbalbaharum/lp-remove-tracker/internal/types"
	"github.com/redis/go-redis/v9"
)

type TrackedAmmStorage struct {
	client *redis.Client
}

const (
	TRACKED_TRIGGER_ONLY = "TRACKED_TRIGGER_ONLY"
	TRACKED_BOTH         = "TRACKED_BOTH"
	PAUSE                = "PAUSE"
	NOT_TRACKED          = "NOT_TRACKED"
)

func SetTracked(client *redis.Client, ammId string, tracker types.Tracker) error {
	ctx := context.Background()

	if tracker.Status != TRACKED_TRIGGER_ONLY && tracker.Status != TRACKED_BOTH && tracker.Status != PAUSE && tracker.Status != NOT_TRACKED {
		return errors.New("invalid tracking status")
	}

	data, err := json.Marshal(tracker)

	if err != nil {
		return err
	}

	if err := client.HSet(ctx, ammId, KEY_TRACKEDAMM, data).Err(); err != nil {
		return err
	}

	return nil
}

func GetTracked(client *redis.Client, ammId string) (*types.Tracker, error) {
	ctx := context.Background()
	data, err := client.HGet(ctx, ammId, KEY_TRACKEDAMM).Result()
	if err != nil {
		if err == redis.Nil {
			return &types.Tracker{
				Status: "NOT_TRACKED",
			}, nil
		}

		return nil, err
	}

	var tracker types.Tracker
	if err := json.Unmarshal([]byte(data), &tracker); err != nil {
		return &types.Tracker{}, err
	}

	switch tracker.Status {
	case TRACKED_TRIGGER_ONLY, TRACKED_BOTH, PAUSE, NOT_TRACKED:
		return &tracker, nil
	default:
		return nil, errors.New("unexpected value in Redis")
	}
}

func GetAllTracked(client *redis.Client) (*[]types.Tracker, error) {
	ctx := context.Background()

	keys, err := client.Keys(ctx, "*").Result()
	if err != nil {
		return nil, err
	}

	var trackers []types.Tracker

	for _, key := range keys {
		data, err := client.HGet(ctx, key, KEY_TRACKEDAMM).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			return nil, err
		}

		var tracker types.Tracker
		if err := json.Unmarshal([]byte(data), &tracker); err != nil {
			return nil, err
		}

		switch tracker.Status {
		case TRACKED_TRIGGER_ONLY, TRACKED_BOTH, PAUSE, NOT_TRACKED:
			trackers = append(trackers, tracker)
		default:
			return nil, errors.New("unexpected value in Redis")
		}
	}

	return &trackers, nil
}
