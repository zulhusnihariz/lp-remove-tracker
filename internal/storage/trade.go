// storage/trade.go
package storage

import (
	"database/sql"
	"fmt"

	"github.com/iqbalbaharum/lp-remove-tracker/internal/types"
)

type TradeStorage struct {
	client *sql.DB
}

func NewTradeStorage(db *sql.DB) *TradeStorage {
	return &TradeStorage{client: db}
}

func (s *TradeStorage) SetTrade(trade *types.Trade) error {
	query := `
			INSERT INTO trades (ammId, mint, action, amount, signature, timestamp)
			VALUES (?, ?, ?, ?, ?, ?)
		`

	_, err := s.client.Exec(
		query,
		trade.AmmId.String(),
		trade.Mint.String(),
		trade.Action,
		trade.Amount,
		trade.Signature,
		trade.Timestamp,
	)

	if err != nil {
		return fmt.Errorf("failed to insert trade: %w", err)
	}

	return nil
}
