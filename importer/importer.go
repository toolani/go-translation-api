package importer

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/petert82/go-translation-api/config"
	"github.com/petert82/go-translation-api/datastore"
	"os"
	"time"
)

func checkFatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func Import(c config.Config) {
	start := time.Now()

	results := make(chan string, 100)
	done := make(chan bool, 1)

	go func() {
		for {
			imported := <-results
			fmt.Println("Imported domain: ", imported)
		}
	}()

	var (
		count int
		stats datastore.Stats
	)
	go func() {
		var db *sqlx.DB
		db, err := sqlx.Connect(c.DB.Driver, c.DB.ConnectionString())
		checkFatal(err)
		ds, err := datastore.New(db, c.DB.Driver)
		checkFatal(err)
		count, err = ds.ImportDir(c.XLIFF.ImportPath, results)
		checkFatal(err)

		stats = ds.Stats

		done <- true
	}()
	<-done

	elapsed := time.Since(start).Seconds()
	fmt.Printf("Imported %v files in %fs\n\n", count, elapsed)

	fmt.Fprintln(os.Stderr, stats)
}
