package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/igorcafe/anyflix/meta"
)

var db *sql.DB

type migration interface {
	migrate() error
}

type migrationString string

func (s migrationString) migrate() error {
	_, err := db.Exec(string(s))
	return err
}

var migrations = []migration{
	// 1
	migrationString(`
CREATE TABLE recent (
	id INTEGER PRIMARY KEY,
	data TEXT NOT NULL,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
)`),
}

func Init() error {
	path, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	db, err = sql.Open("sqlite", filepath.Join(path, "anyflix.db"))
	if err != nil {
		return err
	}

	err = runMigrations()
	if err != nil {
		return err
	}

	return nil
}

func runMigrations() error {
	version, err := getVersion()
	if err != nil {
		return err
	}

	if version == len(migrations) {
		return nil
	}

	err = migrations[version].migrate()
	if err != nil {
		return err
	}

	version++
	err = setVersion(version)
	if err != nil {
		return err
	}

	return runMigrations()
}

func getVersion() (int, error) {
	var version int
	err := db.QueryRow(`PRAGMA user_version`).Scan(&version)
	return version, err
}

func setVersion(version int) error {
	_, err := db.Exec(`PRAGMA user_version = ` + fmt.Sprint(version))
	return err
}

func AddRecent(recent meta.Meta) error {
	_, err := db.Exec(`DELETE FROM recent WHERE json_extract(data, '$.id') = ?`, recent.ID)
	if err != nil {
		return err
	}

	data, err := json.Marshal(recent)
	if err != nil {
		return err
	}

	_, err = db.Exec(`INSERT INTO recent (data) VALUES (?)`, data)
	if err != nil {
		return err
	}

	return nil
}

func ListRecent() ([]meta.Meta, error) {
	rows, err := db.Query(`SELECT data FROM recent ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	recent := []meta.Meta{}

	for rows.Next() {
		var data []byte
		err := rows.Scan(&data)
		if err != nil {
			return nil, err
		}

		rec := meta.Meta{}
		err = json.Unmarshal(data, &rec)
		if err != nil {
			return nil, err
		}

		recent = append(recent, rec)
	}

	return recent, nil
}

func DeleteRecent(id string) error {
	_, err := db.Exec(`DELETE FROM recent WHERE json_extract(data, '$.id') = ?`, id)
	return err
}
