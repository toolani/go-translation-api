package main

import (
	"flag"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/petert82/go-translation-api/config"
	"github.com/petert82/go-translation-api/datastore"
	"log"
	"os"
	"path/filepath"
	"time"
)

var configPath string

func init() {
	defaultConfigPath := filepath.FromSlash("./translation-api.toml")
	flag.StringVar(&configPath, "config", defaultConfigPath, "Full `path` and file name to the config file")
}

func checkFatal(err error, logger *log.Logger) {
	if err != nil {
		logger.Fatalln("Error:", err)
	}
}

func main() {
	logger := log.New(os.Stderr, "", 0)

	flag.Parse()

	config, err := config.Load(configPath)
	checkFatal(err, logger)

	start := time.Now()

	results := make(chan string, 100)
	done := make(chan bool, 1)

	go func() {
		for {
			imported := <-results
			logger.Println("Imported domain: ", imported)
		}
	}()

	var (
		count int
		stats datastore.Stats
	)
	go func() {
		var db *sqlx.DB
		db, err = sqlx.Connect(config.DB.Driver, config.DB.ConnectionString())
		checkFatal(err, logger)
		ds, err := datastore.New(db)
		checkFatal(err, logger)
		count, err = ds.ImportDir(config.XLIFF.ImportPath, results)
		checkFatal(err, logger)

		stats = ds.Stats

		done <- true
	}()
	<-done

	elapsed := time.Since(start).Seconds()
	logger.Printf("Imported %v files in %fs\n\n", count, elapsed)

	logger.Println(stats)
}
