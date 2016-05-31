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
	instructions := `USAGE
    go-translation-api [-config path] [-force] command

DESCRIPTION
    The following commands are available:

        init-db   - Creates or updates the required database table structure for the Translation API.
                    Must be run at least once before any of the other commands.
                    No action is taken if the database is already up to date.
        remove-db - Removes all tables created by the Translation API from the database.
                    All Translation API data will be deleted from the database.
                    Requires that the -force option is provided.
        serve     - Starts the HTTP Translation API server using the settings defined in the config file.
        import    - Imports the content of the XLIFF files from the config file's xliff.import_path into the database.
        export    - Exports translations from the database to XLIFF files in the config file's xliff.export_path.
        help      - Prints this help message.

OPTIONS`
	fmt.Println(instructions)
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
