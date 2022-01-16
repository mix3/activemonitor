package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/subcommands"
	ps "github.com/mitchellh/go-ps"

	_ "github.com/mattn/go-sqlite3"
)

const (
	layoutDate     = "2006-01-02"
	layoutDateTime = "2006-01-02 15:04:05"
)

type Time struct {
	time.Time
}

func (v *Time) Set(str string) (err error) {
	v.Time, err = time.Parse(layoutDate, str)
	return
}

type showCmd struct {
	date     Time
	dbpath   string
	interval int
}

func (*showCmd) Name() string     { return "show" }
func (*showCmd) Synopsis() string { return "show for activemonitor" }
func (*showCmd) Usage() string {
	return `show -date <YYYY-MM-DD> [-dbpath <dbpath>] [-interval <intervalsec>]:
  show for activemonitor
`
}

func (v *showCmd) SetFlags(f *flag.FlagSet) {
	f.Var(&v.date, "date", "date")
	f.IntVar(&v.interval, "interval", defaultInterval, "interval")
}

func (v *showCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	path := dbPath(dbDir, v.date)
	if _, err := os.Stat(path); err != nil {
		log.Println(err)
		return subcommands.ExitFailure
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		log.Println(err)
		return subcommands.ExitFailure
	}
	defer db.Close()

	var (
		sTime = v.date.Add(time.Hour * 5)
		eTime = sTime.Add(time.Hour*24 + time.Second*-1)
		sStr  = sTime.Format(layoutDateTime)
		eStr  = eTime.Format(layoutDateTime)
	)
	rows, err := db.Query(`
SELECT
	time,
	COUNT(time) AS count
FROM (
	SELECT DATETIME(strftime('%s', time) / ? * ?, 'unixepoch') AS time
	FROM receive
	WHERE time BETWEEN ? AND ?
) t
GROUP BY time
`, v.interval, v.interval, sStr, eStr)
	if err != nil {
		log.Println(err)
		return subcommands.ExitFailure
	}
	defer rows.Close()

	summary := map[string]int{}
	for rows.Next() {
		var (
			time  string
			count int
		)
		rows.Scan(&time, &count)
		summary[time] = count
	}

	for s := sTime; s.Before(eTime); s = s.Add(time.Duration(v.interval) * time.Second) {
		var (
			k = s.Format("2006-01-02 15:04:05")
			v = ""
		)
		if c, ok := summary[k]; ok {
			v = strings.Repeat("*", c)
		}
		fmt.Printf("[%s]: %s\n", k, v)
	}

	return subcommands.ExitSuccess
}

type recCmd struct {
	dbDir string
}

func (*recCmd) Name() string     { return "rec" }
func (*recCmd) Synopsis() string { return "rec for activemonitor" }
func (*recCmd) Usage() string {
	return `rec:
  rec for activemonitor
`
}

func (r *recCmd) SetFlags(f *flag.FlagSet) {}

func (v *recCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	processes, err := ps.Processes()
	if err != nil {
		log.Println(err)
		return subcommands.ExitFailure
	}

	for _, p := range processes {
		if strings.Contains(p.Executable(), "ScreenSaverEngin") {
			return subcommands.ExitSuccess
		}
	}

	db, err := sql.Open("sqlite3", dbPath(v.dbDir, currentDate()))
	if err != nil {
		log.Println(err)
		return subcommands.ExitFailure
	}
	defer db.Close()

	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS receive (
	time DATETIME CHECK (time like '____-__-__ __:__:__') PRIMARY KEY
);
INSERT INTO receive VALUES (?);
`, now.Format(layoutDateTime))
	if err != nil {
		log.Println(err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}

func main() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(&showCmd{date: currentDate()}, "")
	subcommands.Register(&recCmd{dbDir: dbDir}, "")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}

const (
	envDateTime = "NOW"
	envDBDir    = "DB_DIR"

	defaultInterval = 300
)

var (
	now   = time.Now()
	dbDir = filepath.Join(os.Getenv("HOME"), ".activemonitor")
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
}

func currentDate() Time {
	t := now.Add(time.Hour * -5)
	t = t.Truncate(time.Hour).Add(-time.Duration(t.Hour()) * time.Hour)
	return Time{t}
}

func dbPath(dir string, date Time) string {
	return filepath.Join(dir, date.Format("20060102")+".db")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
