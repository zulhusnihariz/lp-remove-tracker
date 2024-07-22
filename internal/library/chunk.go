package bot

import (
	"log"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/adapter"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/storage"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/types"
)

func SetTokenChunk(ammId *solana.PublicKey, chunk types.TokenChunk) error {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get initialize redis instance: %v", err)
		return err
	}

	storage.SetChunk(redisClient, ammId.String(), chunk)

	return nil
}

func GetTokenChunk(ammId *solana.PublicKey) (types.TokenChunk, error) {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get initialize redis instance: %v", err)
		return types.TokenChunk{}, err
	}

	chunk, err := storage.GetChunk(redisClient, ammId.String())
	if err != nil {
		return types.TokenChunk{}, err
	}

	return *chunk, nil
}
