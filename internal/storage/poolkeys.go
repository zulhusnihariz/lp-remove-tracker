package storage

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/iqbalbaharum/go-solana-mev-bot/internal/liquidity"
	"github.com/redis/go-redis/v9"
)

type PoolKeysStorage struct {
	client *redis.Client
}

func SetPoolKeys(client *redis.Client, pKey liquidity.RaydiumPoolKeys) error {
	ctx := context.Background()

	data, err := json.Marshal(pKey)
	if err != nil {
		return err
	}

	if err := client.HSet(ctx, KEY_POOLKEYS, pKey.ID, data).Err(); err != nil {
		return err
	}

	return nil
}

func GetPoolKeys(client *redis.Client, ammId string) (liquidity.RaydiumPoolKeys, error) {
	ctx := context.Background()
	data, err := client.HGet(ctx, KEY_POOLKEYS, ammId).Result()
	if err != nil {
		if err == redis.Nil {
			return liquidity.RaydiumPoolKeys{}, errors.New("key not found")
		}
		return liquidity.RaydiumPoolKeys{}, err
	}

	var pKey liquidity.RaydiumPoolKeys
	if err := json.Unmarshal([]byte(data), &pKey); err != nil {
		return liquidity.RaydiumPoolKeys{}, err
	}

	return pKey, nil
}
