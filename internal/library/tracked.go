package bot

import (
	"log"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/go-arbi-bot/internal/adapter"
	"github.com/iqbalbaharum/go-arbi-bot/internal/storage"
)

func trackedInit() {

}

func TrackedAmm(ammId *solana.PublicKey) {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get initialize redis instance: %v", err)
	}

	storage.SetTracked(redisClient, ammId.String(), storage.TRACKED)
}

func PauseAmmTracking(ammId *solana.PublicKey) {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get initialize redis instance: %v", err)
	}

	storage.SetTracked(redisClient, ammId.String(), storage.PAUSE)
}

func UntrackedAmm(ammId *solana.PublicKey) {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get initialize redis instance: %v", err)
	}

	storage.SetTracked(redisClient, ammId.String(), storage.NOT_TRACKED)
}

func GetAmmTrackingStatus(ammId *solana.PublicKey) (string, error) {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get initialize redis instance: %v", err)
	}

	isTracked, error := storage.GetTracked(redisClient, ammId.String())
	if error != nil {
		return "", error
	}

	return isTracked, nil
}
