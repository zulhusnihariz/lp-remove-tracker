package storage

import "database/sql"

var (
	Trade *tradeStorage
)

func Init(client *sql.DB) {
	Trade = NewTradeStorage(client)
}
