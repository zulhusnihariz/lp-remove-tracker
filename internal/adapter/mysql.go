package adapter

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/config"
	db "github.com/iqbalbaharum/lp-remove-tracker/internal/database"
)

var (
	Database  *db.Database
	mysqlOnce sync.Once
)

func InitSqlClient(dsn string) error {
	if dsn == "" {
		return errors.New("MySQL DSN is empty")
	}

	var initError error

	var client *sql.DB
	var err error
	mysqlOnce.Do(func() {
		client, err = sql.Open("mysql", dsn)
		if err != nil {
			initError = fmt.Errorf("failed to connect to MySQL: %v", err)
			return
		}

		if err := client.Ping(); err != nil {
			initError = fmt.Errorf("failed to ping MySQL: %v", err)
			return
		}
	})

	if initError != nil {
		return initError
	}

	db, err := db.NewDatabase(client, config.MySqlDbName)
	if err != nil {
		return err
	}

	err = db.CreateDatabaseAndTables()
	if err != nil {
		return err
	}

	Database = db

	return nil
}

func GetMySQLClient() (*sql.DB, error) {
	if Database == nil {
		return nil, errors.New("MySQL client is not initialized. call InitMySQLClient first")
	}
	return Database.MysqlClient, nil
}
