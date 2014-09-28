package main

import (
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/petert82/trans-api/datastore"
	"os"
)

func check(e error) {
	if e != nil {
		fmt.Println(e)
		os.Exit(1)
	}
}

func parseArgs(args []string) (dbPath string, importPath string, err error) {
	if len(args) < 2 {
		return "", "", errors.New("Usage:\n  transimporter DB_PATH IMPORT_PATH")
	}

	return args[0], args[1], nil
}

func main() {
	dbFile, importPath, err := parseArgs(os.Args[1:])
	check(err)

	results := make(chan string)
	done := make(chan bool, 1)

	go func() {
		for {
			imported := <-results
			fmt.Println("Imported domain: ", imported)
		}
	}()

	go func() {
		var db *sqlx.DB
		db, err = sqlx.Connect("sqlite3", dbFile)
		check(err)
		ds := datastore.New(db)
		count, err := ds.ImportDir(importPath, results)
		check(err)

		fmt.Printf("Imported %v files\n", count)
		done <- true
	}()

	<-done

	// err = ds.ImportDomain(&xliff.File.XliffDomain)
	// if err == nil {
	// 	fmt.Println("ok")
	// } else {
	// 	fmt.Println("NOK: ", err.Error())
	// }
}
