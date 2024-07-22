package storage

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/iqbalbaharum/go-solana-mev-bot/internal/types"
	"github.com/redis/go-redis/v9"
)

type TokenChunkStorage struct {
	client *redis.Client
}

func SetChunk(client *redis.Client, ammId string, chunk types.TokenChunk) error {
	ctx := context.Background()

	data, err := json.Marshal(chunk)

	if err != nil {
		return err
	}

	if err := client.HSet(ctx, ammId, KEY_CHUNK, data).Err(); err != nil {
		return err
	}

	return nil
}

func GetChunk(client *redis.Client, ammId string) (*types.TokenChunk, error) {
	ctx := context.Background()
	chunk, err := client.HGet(ctx, ammId, KEY_CHUNK).Result()

	if err != nil {
		if err == redis.Nil {
			return &types.TokenChunk{}, errors.New("key not found")
		}
		return &types.TokenChunk{}, err
	}

	var tokenChunk types.TokenChunk
	if err := json.Unmarshal([]byte(chunk), &tokenChunk); err != nil {
		return &types.TokenChunk{}, err
	}

	return &tokenChunk, nil
}
