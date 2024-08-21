package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

type Database struct {
	dbName      string
	MysqlClient *sql.DB
}

type MySQLFilter struct {
	Query []MySQLQuery
}

type MySQLQuery struct {
	Column string
	Op     string
	Query  string
}

type Column struct {
	Field string
	Type  string
}

func NewDatabase(client *sql.DB, dbName string) (*Database, error) {
	return &Database{
		dbName:      dbName,
		MysqlClient: client,
	}, nil
}

func (d *Database) CreateDatabaseAndTables() error {

	createDatabase := `CREATE DATABASE IF NOT EXISTS ` + d.dbName

	_, err := d.MysqlClient.Exec(createDatabase)

	if err != nil {
		return fmt.Errorf(fmt.Sprintf("Failed to create db %s: %v", d.dbName, err))
	}

	useDatabase := `USE ` + d.dbName

	_, err = d.MysqlClient.Exec(useDatabase)

	if err != nil {
		return fmt.Errorf(fmt.Sprintf("Failed to use db %s: %v", d.dbName, err))
	}

	path := "./migrations/"

	entries, err := os.ReadDir(path)

	if err != nil {
		log.Fatal(err)
	}

	for _, e := range entries {
		c, err := os.ReadFile(fmt.Sprintf(path + e.Name()))

		if err != nil {
			log.Fatal(err)
		}

		_, err = d.MysqlClient.Exec(string(c))

		if err != nil {
			log.Fatal(err)
		}
	}

	return nil
}
