package bot

import (
	"log"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/adapter"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/storage"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/types"
)

func trackedInit() {

}

func TrackedAmm(ammId *solana.PublicKey, triggerOnly bool) {
	redisClient, err := adapter.GetRedisClient(4)
	if err != nil {
		log.Fatalf("Failed to get initialize redis instance: %v", err)
	}

	var tracker types.Tracker = types.Tracker{}
	tracker.AmmId = ammId

	if triggerOnly {
		tracker.Status = storage.TRACKED_TRIGGER_ONLY
	} else {
		tracker.Status = storage.TRACKED_BOTH
		tracker.LastUpdated = time.Now().Unix()
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
