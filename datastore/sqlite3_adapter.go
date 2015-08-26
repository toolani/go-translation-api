package datastore

import (
	"github.com/jmoiron/sqlx"
)

// Sqlite3Adapter provides support for SQLite3 databases.
type Sqlite3Adapter struct{}

func (s Sqlite3Adapter) PostCreate(db *sqlx.DB) (err error) {
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		return err
	}
	// Faster than using default journal file
	_, err = db.Exec("PRAGMA journal_mode = WAL")
	if err != nil {
		return err
	}
	// Default (full) is slower
	_, err = db.Exec("PRAGMA synchronous = NORMAL")
	if err != nil {
		return err
	}

	return nil
}

func (s Sqlite3Adapter) CreateDomainQuery() string {
	return "INSERT INTO domain (name) VALUES (?)"
}

func (s Sqlite3Adapter) CreateStringQuery() string {
	return "INSERT INTO string (name, domain_id) VALUES (?, ?)"
}

func (s Sqlite3Adapter) CreateTranslationQuery() string {
	return "INSERT INTO translation (language_id, content, string_id) VALUES (?, ?, ?)"
}

func (s Sqlite3Adapter) GetAllDomainsQuery() string {
	return "SELECT name FROM domain ORDER BY name"
}

func (s Sqlite3Adapter) GetAllLanguagesQuery() string {
	return "SELECT id, code, name FROM language ORDER BY code"
}

func (s Sqlite3Adapter) GetSingleDomainQuery() string {
	return "SELECT string.id AS stringId, string.name, translation.language_id AS languageId, language.code, translation.id AS translationId, translation.content FROM string INNER JOIN translation ON string.id = translation.string_id INNER JOIN language ON translation.language_id = language.id WHERE string.domain_id = (SELECT id FROM domain where domain.name = ?) ORDER BY string.name"
}

func (s Sqlite3Adapter) GetSingleDomainIdQuery() string {
	return "SELECT id FROM domain WHERE name=?"
}

func (s Sqlite3Adapter) GetSingleLanguageQuery() string {
	return "SELECT id, name, code FROM language WHERE code=?"
}

func (s Sqlite3Adapter) GetSingleStringIdQuery() string {
	return "SELECT id FROM string WHERE name = ? AND domain_id = ?"
}

func (s Sqlite3Adapter) GetSingleTranslationIdQuery() string {
	return "SELECT translation.id FROM string INNER JOIN translation ON string.id = translation.string_id WHERE string.id=? AND language_id=? AND domain_id=?"
}

func (s Sqlite3Adapter) UpdateTranslationQuery() string {
	return "UPDATE translation SET language_id=?, content=?, string_id=? WHERE id=?"
}
