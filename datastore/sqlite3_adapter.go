package datastore

import (
	"database/sql"
	"errors"
	"github.com/jmoiron/sqlx"
)

// Sqlite3Adapter provides support for SQLite3 databases.
type Sqlite3Adapter struct{}

func (s Sqlite3Adapter) EnsureVersionTableExists(db *sqlx.DB) (err error) {
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS "schema_migrations" ("version" INTEGER PRIMARY KEY NOT NULL)`)
	if err != nil {
		return err
	}

	var count int
	err = db.Get(&count, `SELECT COUNT(*) FROM schema_migrations`)
	if err != nil {
		return err
	}
	switch {
	case count == 0:
		_, err = db.Exec(`INSERT INTO schema_migrations (version) VALUES (0)`)
	case count > 1:
		err = errors.New("too many rows in schema_migrations table")
	}

	return err
}

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

func (s Sqlite3Adapter) up() []string {
	return []string{
		// 1
		`
CREATE TABLE "domain" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "name" TEXT UNIQUE
);
CREATE TABLE "language" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "name" TEXT,
    "code" TEXT
);
CREATE INDEX "code" ON "language" ("code");
CREATE TABLE "string" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "name" TEXT,
    "domain_id" INTEGER REFERENCES "domain"("id") ON UPDATE CASCADE ON DELETE CASCADE
);
CREATE INDEX "domain_id" ON "string" ("domain_id");
CREATE INDEX "name" ON "string" ("name");
CREATE TABLE "translation" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "language_id" INTEGER REFERENCES "language"("id") ON UPDATE CASCADE ON DELETE CASCADE,
    "content" TEXT,
    "string_id" INTEGER REFERENCES "string"("id") ON UPDATE CASCADE ON DELETE CASCADE
);
CREATE INDEX "language_id" ON "translation" ("language_id");
CREATE INDEX "string_id" ON "translation" ("string_id");
CREATE INDEX "string_id_language_id" ON "translation" ("language_id","string_id");
INSERT INTO language (name, code) VALUES
    ("German","de"),
    ("English","en"),
    ("Spanish","es"),
    ("French","fr"),
    ("Italian","it"),
    ("Polish","pl"),
    ("German (Austria)","de-at"),
    ("German (Switzerland)","de-ch"),
    ("German (Germany)","de-de"),
    ("English (Australia)","en-au"),
    ("English (Canada)","en-ca"),
    ("English (UK)","en-gb"),
    ("English (Bahrain)","en-bh"),
    ("English (US)","en-us"),
    ("English (South Africa)","en-za"),
    ("French (Canada)","fr-ca"),
    ("Portuguese","pt"),
    ("English (Ireland)","en-ie"),
    ("Czech","cs"),
    ("Hungarian","hu"),
    ("Spanish (US)","es-us");
`,
		// 2
		`INSERT INTO language (code, name) VALUES ("nl", "Dutch")`,
	}
}

func (s Sqlite3Adapter) down() []string {
	return []string{
		// 1
		`
DROP TABLE translation;
DROP TABLE string;
DROP TABLE language;
DROP TABLE domain;
`,
		// 2
		`DELETE FROM language WHERE code = "nl"`,
	}
}

func (s Sqlite3Adapter) MigrateUp(db *sqlx.DB) (version int64, err error) {
	startVer, err := s.version(db)
	if err != nil {
		return version, err
	}

	for i, query := range s.up() {
		migTo := int64(i + 1)
		if migTo <= startVer {
			version = migTo
			continue
		}

		_, err = db.Exec(query)
		if err != nil {
			return version, err
		}

		err = s.updateVersion(migTo, db)
		if err != nil {
			return version, err
		}

		version = migTo
	}

	return version, err
}

func (s Sqlite3Adapter) MigrateDown(db *sqlx.DB) (version int64, err error) {
	startVer, err := s.version(db)
	if err != nil {
		return version, err
	}

	down := s.down()
	for i := len(down) - 1; i >= 0; i-- {
		query := down[i]
		migVer := int64(i + 1) // The version of the Down migration we will apply
		migTo := int64(i)      // The version we will end up at

		// Skip migrations for newer versions
		if migVer > startVer {
			version = startVer
			continue
		}

		_, err = db.Exec(query)
		if err != nil {
			return version, err
		}

		err = s.updateVersion(migTo, db)
		if err != nil {
			return version, err
		}

		version = migTo
	}

	return version, err
}

func (s Sqlite3Adapter) SupportsLastInsertId() bool {
	return true
}

func (s Sqlite3Adapter) CreateDomainQuery() string {
	return "INSERT INTO domain (name) VALUES (?)"
}

func (s Sqlite3Adapter) CreateLanguageQuery() string {
	return "INSERT INTO language (code, name) VALUES (?, ?)"
}

func (s Sqlite3Adapter) CreateStringQuery() string {
	return "INSERT INTO string (name, domain_id) VALUES (?, ?)"
}

func (s Sqlite3Adapter) CreateTranslationQuery() string {
	return "INSERT INTO translation (language_id, content, string_id) VALUES (?, ?, ?)"
}

func (s Sqlite3Adapter) DeleteTranslationQuery() string {
	return "DELETE FROM translation WHERE id = ?"
}

func (s Sqlite3Adapter) GetAllDomainsQuery() string {
	return "SELECT name FROM domain ORDER BY name"
}

func (s Sqlite3Adapter) GetAllLanguagesQuery() string {
	return "SELECT id, code, name FROM language ORDER BY code"
}

func (s Sqlite3Adapter) GetSingleDomainQuery() string {
	return "SELECT string.id AS string_id, string.name, translation.language_id AS language_id, language.code, translation.id AS translation_id, translation.content FROM string INNER JOIN translation ON string.id = translation.string_id INNER JOIN language ON translation.language_id = language.id WHERE string.domain_id = (SELECT id FROM domain where domain.name = ?) ORDER BY string.name"
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

func (s Sqlite3Adapter) version(db *sqlx.DB) (version int64, err error) {
	row := db.QueryRow("SELECT version FROM schema_migrations")
	err = row.Scan(&version)
	switch {
	case err == sql.ErrNoRows:
		return 0, nil
	case err != nil:
		return 0, err
	default:
		return version, nil
	}
}

func (s Sqlite3Adapter) updateVersion(version int64, db *sqlx.DB) (err error) {
	_, err = db.Exec("UPDATE schema_migrations SET version = ?", int64(version))

	return err
}
