package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/subcommands"
	ps "github.com/mitchellh/go-ps"

	_ "github.com/mattn/go-sqlite3"
)

type dateValue struct {
	time.Time
}

func (v *dateValue) Set(str string) (err error) {
	v.Time, err = time.Parse("2006-01-02", str)
	return
}

type showCmd struct {
	date     dateValue
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
	var (
		defaultDB       = os.Getenv("HOME") + "/.activemonitor.db"
		defaultInterval = 300
	)
	f.Var(&v.date, "date", "date")
	f.StringVar(&v.dbpath, "dbpath", defaultDB, "dbpath")
	f.IntVar(&v.interval, "interval", defaultInterval, "interval")
}

func (v *showCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	db, err := sql.Open("sqlite3", v.dbpath)
	if err != nil {
		log.Println(err)
		return subcommands.ExitFailure
	}
	defer db.Close()

	var (
		sTime = v.date.Add(time.Hour * 5)
		eTime = sTime.Add(time.Hour*24 + time.Second*-1)
		sStr  = sTime.Format("2006-01-02 15:04:05")
		eStr  = eTime.Format("2006-01-02 15:04:05")
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
	dbpath string
}

func (*recCmd) Name() string     { return "rec" }
func (*recCmd) Synopsis() string { return "rec for activemonitor" }
func (*recCmd) Usage() string {
	return `rec [-dbpath <dbpath>]:
  rec for activemonitor
`
}

func (r *recCmd) SetFlags(f *flag.FlagSet) {
	var (
		defaultDB = os.Getenv("HOME") + "/.activemonitor.db"
	)
	f.StringVar(&r.dbpath, "dbpath", defaultDB, "dbpath")
}

func (v *recCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	processes, err := ps.Processes()
	if err != nil {
		log.Println(err)
		return subcommands.ExitFailure
	}

	for _, p := range processes {
		if strings.Contains(p.Executable(), "ScreenSaverEngine") {
			return subcommands.ExitSuccess
		}
	}

	db, err := sql.Open("sqlite3", v.dbpath)
	if err != nil {
		log.Println(err)
		return subcommands.ExitFailure
	}
	defer db.Close()

	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS receive (
	time DATETIME CHECK (time like '____-__-__ __:__:__') PRIMARY KEY
);
INSERT INTO receive VALUES (DATETIME('now', '+9 hour'));
`)
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
	subcommands.Register(&recCmd{}, "")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}

func currentDate() dateValue {
	t := time.Now()
	t = t.Add(time.Hour * -5)
	t = t.Truncate(time.Hour).Add(-time.Duration(t.Hour()) * time.Hour)
	return dateValue{t}
}
