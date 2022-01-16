package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/sync/errgroup"
)

const (
	layoutDate     = "2006-01-02"
	layoutDateTime = "2006-01-02 15:04:05"

	envDBPath   = "DB_PATH"
	envDateTime = "NOW"
	envDBDir    = "DB_DIR"

	defaultInterval = 300
)

var (
	now    = time.Now()
	dbDir  = filepath.Join(os.Getenv("HOME"), ".activemonitor")
	dbpath = filepath.Join(os.Getenv("HOME"), ".activemonitor.db")
)

func init() {
	if v, ok := os.LookupEnv(envDateTime); ok {
		_now, err := time.Parse(layoutDateTime, v)
		if err != nil {
			panic(err)
		}
		now = _now
	}
	if v, ok := os.LookupEnv(envDBDir); ok {
		dbDir = v
	}
	if v, ok := os.LookupEnv(envDBPath); ok {
		dbpath = v
	}
}

func dbPath(dir string, date time.Time) string {
	return filepath.Join(dir, date.Format("20060102")+".db")
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if _, err := os.Stat(dbpath); err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		return err
	}
	defer db.Close()

	ret := map[int64][]string{}

	rows, err := db.Query(`SELECT strftime('%s', date(time, '-5 hours'), 'utc'), strftime('%Y-%m-%d %H:%M:%S', time) FROM receive`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			dateepoch int64
			datetime  string
		)
		if err := rows.Scan(&dateepoch, &datetime); err != nil {
			return err
		}
		ret[dateepoch] = append(ret[dateepoch], datetime)
	}

	sem := make(chan struct{}, 10)
	g := &errgroup.Group{}
	for k, _vs := range ret {
		sem <- struct{}{}
		date := time.Unix(k, 0)
		vs := _vs
		g.Go(func() error {
			defer func() { <-sem }()
			db, err := sql.Open("sqlite3", dbPath(dbDir, date))
			if err != nil {
				return err
			}
			defer db.Close()
			for _, v := range vs {
				_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS receive (
	time DATETIME CHECK (time like '____-__-__ __:__:__') PRIMARY KEY
);
INSERT INTO receive VALUES (?);
`, v)
				if err != nil {
					return err
				}
			}
			return nil
		})
	}
	return g.Wait()
}
