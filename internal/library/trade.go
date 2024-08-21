package bot

import (
	"log"

	"github.com/iqbalbaharum/lp-remove-tracker/internal/adapter"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/storage"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/types"
)

func SetTrade(trade *types.Trade) error {
	db, err := adapter.GetMySQLClient()
	if err != nil {
		log.Printf("Failed to get initialize mysql instance: %v", err)
		return err
	}

	tradeStorage := storage.NewTradeStorage(db)
	return tradeStorage.SetTrade(trade)
}
