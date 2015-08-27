package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/petert82/go-translation-api/config"
	"github.com/petert82/go-translation-api/datastore"
	"os"
	"strings"
)

type Command interface {
	Run(config.Config)
}

type CommandFunc func(config.Config)

func (f CommandFunc) Run(c config.Config) {
	f(c)
}

const (
	cmdMissing      = "missing"
	cmdUnrecognised = "unrecognised"
	cmdHelp         = "help"
	cmdImport       = "import"
	cmdInitDb       = "init-db"
	cmdServe        = "serve"
)

// Gets list of available commands
func availableCommands() []string {
	return []string{cmdHelp, cmdImport, cmdInitDb, cmdServe}
}

// initDb initializes the database with all necessary tables.
func initDb(c config.Config) {
	var db *sqlx.DB
	db, err := sqlx.Connect(c.DB.Driver, c.DB.ConnectionString())
	checkFatal(err)
	ds, err := datastore.New(db, c.DB.Driver)
	checkFatal(err)

	dbVersion, err := ds.MigrateUp()
	if err != nil {
		fmt.Println(err)
		checkFatal(errors.New(fmt.Sprintf("Could complete database migration, last applied version was %v", dbVersion)))
	}

	fmt.Println("Successfully migrated the database to version", dbVersion)
}

// Prints a normal usage message.
func printUsage(c config.Config) {
	flag.PrintDefaults()
}

// Prints a usage message indicating that a command must be given.
func printMissingCommandUsage(c config.Config) {
	fmt.Fprintf(os.Stderr, "No command given. Command can be one of: %v\n\n", strings.Join(availableCommands(), ", "))
	printUsage(c)
}

// Prints a usage message indicating that the given command was not recognised.
func printUnrecognisedCommandUsage(cmd string) CommandFunc {
	return func(c config.Config) {
		fmt.Fprintf(os.Stderr, "Command '%v' not recognised. Command must be one of: %v\n\n", os.Args[1], strings.Join(availableCommands(), ", "))
		printUsage(c)
	}
}
