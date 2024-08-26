package bot

import (
	"log"
	"sync"
	"time"

	"github.com/iqbalbaharum/lp-remove-tracker/internal/adapter"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/storage"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/types"
)

var dbMutex sync.Mutex

func SetTrade(trade *types.Trade) error {
	db, err := adapter.GetMySQLClient()
	if err != nil {
		log.Printf("Failed to get initialize mysql instance: %v", err)
		return err
	}

	dbMutex.Lock()
	defer dbMutex.Unlock()

	tradeStorage := storage.NewTradeStorage(db)

	trade.Timestamp = time.Now().Unix()
	err = tradeStorage.Set(trade)
	if err != nil {
		log.Printf("Failed to set trade: %v", err)
	}

	return nil
}
