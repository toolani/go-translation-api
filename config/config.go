/*
Package config implements TOML config file handling for the translation API.

Normally it will be used by simply passing a config file name to the Load function to obtain a
Config struct.
*/
package config

import (
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"os"
	"path/filepath"
)

const (
	DbDriverSqlite3 = "sqlite3"
)

// Config represents the parsed configuration for the translation API.
type Config struct {
	DB     DbConfig     `toml:"database"`
	Server ServerConfig `toml:"server"`
	XLIFF  XliffConfig  `toml:"xliff"`
}

// valid checks if the Config is valid in its current state.
func (c *Config) valid() error {
	if c.DB.Driver != DbDriverSqlite3 {
		return errors.New(fmt.Sprintf("config: invalid database.driver value. (Must be one of: '%v')", DbDriverSqlite3))
	}
	if len(c.DB.File) == 0 {
		return errors.New("config: missing database.file value")
	}
	if c.Server.Port < 0 {
		return errors.New("config: server.port is invalid")
	}
	if len(c.XLIFF.ImportPath) == 0 {
		return errors.New("config: missing xliff.import_path value")
	}
	if len(c.XLIFF.ExportPath) == 0 {
		return errors.New("config: missing xliff.export_path value")
	}
	if _, err := os.Stat(filepath.FromSlash(c.XLIFF.ImportPath)); os.IsNotExist(err) {
		return errors.New("xliff: import_path does not exist")
	}
	return nil
}

// DbConfig contains Database connection configuration.
type DbConfig struct {
	// Must currently be 'sqlite3'
	Driver string
	// When driver is sqlite3, this is the path to the database file
	File     string
	Host     string
	Port     int
	Name     string
	User     string
	Password string
}

// ServerConfig contains HTTP server configuration.
type ServerConfig struct {
	// Port that the server should run on.
	Port int
}

// XliffConfig contains XLIFF import/export configuration.
type XliffConfig struct {
	// Path to import XLIFF files from
	ImportPath string `toml:"import_path"`
	// Path to export XLIFF files to
	ExportPath string `toml:"export_path"`
}

// Gets a connection string for this config.
func (d *DbConfig) ConnectionString() string {
	return d.File
	// username:password@protocol(address)/dbname?param=value
	// return fmt.Sprintf("%v:%v@tcp(%v:%v)/%v?parseTime=true", d.User, d.Password, d.Host, d.Port, d.Name)
}

// Creates a new Config with some default values.
func new() Config {
	c := Config{
		DB: DbConfig{
			Driver: "sqlite3",
			File:   filepath.FromSlash("./translations.db"),
		},
		Server: ServerConfig{
			Port: 8181,
		},
		XLIFF: XliffConfig{
			ImportPath: filepath.FromSlash("./xliff-in"),
			ExportPath: filepath.FromSlash("./xliff-out"),
		},
	}
	return c
}

// Loads config from a TOML file and checks its validity.
func Load(file string) (Config, error) {
	conf := new()
	_, err := toml.DecodeFile(file, &conf)
	if err != nil {
		return conf, err
	}

	if err = conf.valid(); err != nil {
		return conf, err
	}

	return conf, nil
}
