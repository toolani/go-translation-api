/*
A tool for managing a database of translations and providing the means to import and export these
translations to and from XLIFF files.

Various program settings are controlled by a TOML config file, which must be available for the
program to run. By default, the program will look for a file called 'translation-api.toml' in the
same directory as its binary.

The program must be run with a 'command' argument to indicate what you would like it to do.
Available commands are:

  - import: Imports translations from XLIFF files in the xliff 'import_path' given in the config file.
  - serve: Starts an HTTP server providing a JSON API for accessing and modifying the translation data.
  - help: Prints usage instructions
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
	"strings"
)

var (
	configPath string
)

const (
	cmdMissing      = "missing"
	cmdUnrecognised = "unrecognised"
	cmdHelp         = "help"
	cmdImport       = "import"
	cmdServe        = "serve"
)

func init() {
	defaultConfigPath := filepath.FromSlash("./translation-api.toml")
	flag.StringVar(&configPath, "config", defaultConfigPath, "Full `path` and file name to the config file")
}

type Command interface {
	Run(config.Config)
}

type CommandFunc func(config.Config)

func (f CommandFunc) Run(c config.Config) {
	f(c)
}

// Gets list of available commands
func availableCommands() []string {
	return []string{cmdImport, cmdServe, cmdHelp}
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
	case cmdServe:
		return cmdServe
	}

	return cmdUnrecognised
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

func main() {
	flag.Parse()
	config, cfgErr := config.Load(configPath)
	var command = parseArgs(os.Args[1:])

	var commandFunc = CommandFunc(printMissingCommandUsage)
	switch command {
	case cmdUnrecognised:
		commandFunc = printUnrecognisedCommandUsage(command)
	case cmdImport:
		commandFunc = CommandFunc(importer.Import)
	case cmdServe:
		commandFunc = CommandFunc(server.Serve)
	}

	// Invalid config only matters for non-'help' commands
	if command != cmdUnrecognised && command != cmdMissing && command != cmdHelp {
		checkFatal(cfgErr)
	}

	commandFunc.Run(config)
}
