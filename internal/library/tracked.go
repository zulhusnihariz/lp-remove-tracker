package bot

import (
	"log"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/adapter"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/config"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/storage"
)

func trackedInit() {

}

func RegisterAmm(ammId *solana.PublicKey) {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get initialize redis instance: %v", err)
	}

	if config.FlagPoolTracked {
		storage.SetTracked(redisClient, ammId.String(), true)
	}
}

func UnregisterAmm(ammId *solana.PublicKey) {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get initialize redis instance: %v", err)
	}

	if config.FlagPoolTracked {
		storage.SetTracked(redisClient, ammId.String(), false)
	}
}

func GetAmmTrackingStatus(ammId *solana.PublicKey) (bool, error) {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get initialize redis instance: %v", err)
	}

	isTracked, error := storage.GetTracked(redisClient, ammId.String())
	if error != nil {
		return false, error
	}

	return isTracked, nil
}
