package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/toolani/go-translation-api/config"
	"github.com/toolani/go-translation-api/datastore"
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
	cmdExport       = "export"
	cmdHelp         = "help"
	cmdImport       = "import"
	cmdInitDb       = "init-db"
	cmdRemoveDb     = "remove-db"
	cmdServe        = "serve"
)

// Gets list of available commands
func availableCommands() []string {
	return []string{cmdHelp, cmdExport, cmdImport, cmdInitDb, cmdRemoveDb, cmdServe}
}

func getDatastore(c config.Config) (ds *datastore.DataStore) {
	var db *sqlx.DB
	db, err := sqlx.Connect(c.DB.Driver, c.DB.ConnectionString())
	checkFatal(err)
	ds, err = datastore.New(db, c.DB.Driver)
	checkFatal(err)

	return ds
}

// initDb initializes the database with all necessary tables.
func initDb(c config.Config) {
	ds := getDatastore(c)

	dbVersion, err := ds.MigrateUp()
	if err != nil {
		fmt.Println(err)
		checkFatal(errors.New(fmt.Sprintf("Could complete database migration, last applied version was %v", dbVersion)))
	}

	fmt.Println("Successfully migrated the database to version", dbVersion)
}

// Exports all translation domains to XLIFF
func export(c config.Config) {
	ds := getDatastore(c)

	domains, err := ds.GetDomainList()
	checkFatal(err)

	fmt.Printf("Exporting %v translation domains to: %v\n", len(domains), c.XLIFF.ExportPath)

	for _, dom := range domains {
		err = ds.ExportDomain(dom.Name(), c.XLIFF.ExportPath)
		checkFatal(err)

		fmt.Printf("Exported domain '%v'\n", dom.Name())
	}
}

// printMustForceToRemoveDb prints usage for the remove-db command
func printMustForceToRemoveDb(c config.Config) {
	fmt.Fprintln(os.Stderr, "The remove-db command requires the '--force' flag")
}

// removeDb removes any tables added by initDb
func removeDb(c config.Config) {
	ds := getDatastore(c)

	dbVersion, err := ds.MigrateDown()
	if err != nil {
		fmt.Println(err)
		checkFatal(errors.New(fmt.Sprintf("Could complete database removal, last applied version was %v", dbVersion)))
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
