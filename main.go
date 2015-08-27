/*
A tool for managing a database of translations and providing the means to import and export these
translations to and from XLIFF files.

Various program settings are controlled by a TOML config file, which must be available for the
program to run. By default, the program will look for a file called 'translation-api.toml' in the
same directory as its binary.

The program must be run with a 'command' argument to indicate what you would like it to do.
Available commands are:

  - help: Prints usage instructions
  - import: Imports translations from XLIFF files in the xliff 'import_path' given in the config file.
  - init-db: Ensures that the database contains all necessary tables. Safe to be run multiple times.
  - remove-db: Removes all translation API data from the database (requires the --force flag).
  - serve: Starts an HTTP server providing a JSON API for accessing and modifying the translation data.
*/
package main

import (
	"flag"
	"fmt"
	"github.com/petert82/go-translation-api/config"
	"github.com/petert82/go-translation-api/importer"
	"github.com/petert82/go-translation-api/server"
	"os"
	"path/filepath"
)

var (
	configPath string
	force      bool
)

func init() {
	defaultConfigPath := filepath.FromSlash("./translation-api.toml")
	flag.StringVar(&configPath, "config", defaultConfigPath, "Full `path` and file name to the config file")
	flag.BoolVar(&force, "force", false, "Use to allow potentially destructive changes")
}

func checkFatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// Converts os.Args to one of the cmd* constants.
func parseArgs(args []string) (command string) {
	if len(args) < 1 {
		return cmdMissing
	}

	switch args[0] {
	case cmdHelp:
		return cmdHelp
	case cmdImport:
		return cmdImport
	case cmdInitDb:
		return cmdInitDb
	case cmdRemoveDb:
		return cmdRemoveDb
	case cmdServe:
		return cmdServe
	}

	return cmdUnrecognised
}

func main() {
	flag.Parse()
	config, cfgErr := config.Load(configPath)
	var command = parseArgs(flag.Args())

	var commandFunc = CommandFunc(printMissingCommandUsage)
	switch command {
	case cmdUnrecognised:
		commandFunc = printUnrecognisedCommandUsage(command)
	case cmdHelp:
		commandFunc = CommandFunc(printUsage)
	case cmdImport:
		commandFunc = CommandFunc(importer.Import)
	case cmdInitDb:
		commandFunc = CommandFunc(initDb)
	case cmdRemoveDb:
		// Force flag must be set to _really_ remove the database
		if force {
			commandFunc = CommandFunc(removeDb)
		} else {
			commandFunc = CommandFunc(printMustForceToRemoveDb)
		}
	case cmdServe:
		commandFunc = CommandFunc(server.Serve)
	}

	// Invalid config only matters for non-'help' commands
	if command != cmdUnrecognised && command != cmdMissing && command != cmdHelp {
		checkFatal(cfgErr)
	}

	commandFunc.Run(config)
}
