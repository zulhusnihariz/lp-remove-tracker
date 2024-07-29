package bot

import (
	"log"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/go-arbi-bot/internal/adapter"
	"github.com/iqbalbaharum/go-arbi-bot/internal/storage"
	"github.com/iqbalbaharum/go-arbi-bot/internal/types"
)

func trackedInit() {

}

func TrackedAmm(ammId *solana.PublicKey, triggerOnly bool) {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get initialize redis instance: %v", err)
	}

	var status string
	if triggerOnly {
		status = storage.TRACKED_TRIGGER_ONLY
	} else {
		status = storage.TRACKED_BOTH
	}

	var tracker types.Tracker = types.Tracker{
		AmmId:       ammId,
		Status:      status,
		LastUpdated: time.Now().Unix(),
	}

	storage.SetTracked(redisClient, ammId.String(), tracker)
}

func PauseAmmTracking(ammId *solana.PublicKey) {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get initialize redis instance: %v", err)
	}

	var tracker types.Tracker = types.Tracker{
		AmmId:       ammId,
		Status:      storage.PAUSE,
		LastUpdated: time.Now().Unix(),
	}

	storage.SetTracked(redisClient, ammId.String(), tracker)
}

func UntrackedAmm(ammId *solana.PublicKey) {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get initialize redis instance: %v", err)
	}

	var tracker types.Tracker = types.Tracker{
		AmmId:       ammId,
		Status:      storage.NOT_TRACKED,
		LastUpdated: time.Now().Unix(),
	}

	storage.SetTracked(redisClient, ammId.String(), tracker)
}

func GetAmmTrackingStatus(ammId *solana.PublicKey) (*types.Tracker, error) {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get initialize redis instance: %v", err)
	}

	tracker, error := storage.GetTracked(redisClient, ammId.String())
	if error != nil {
		return nil, error
	}

	return tracker, nil
}

func GetAllTrackedAmm() (*[]types.Tracker, error) {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get initialize redis instance: %v", err)
	}

	trackers, error := storage.GetAllTracked(redisClient)
	if error != nil {
		return nil, error
	}

	return trackers, nil
}
