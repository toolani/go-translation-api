package datastore

import (
	"database/sql"
	"errors"
	"github.com/jmoiron/sqlx"
)

// PostgresAdapter provides support for PostgreSQL databases.
type PostgresAdapter struct{}

func (a PostgresAdapter) EnsureVersionTableExists(db *sqlx.DB) (err error) {
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version integer PRIMARY KEY NOT NULL)`)
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

func (a PostgresAdapter) PostCreate(db *sqlx.DB) (err error) {
	return nil
}

func (a PostgresAdapter) up() []string {
	return []string{
		// 1
		`
CREATE TABLE domain (
    id SERIAL PRIMARY KEY,
    name varchar UNIQUE
);
CREATE TABLE language (
    id SERIAL PRIMARY KEY,
    name varchar,
    code varchar UNIQUE
);
CREATE INDEX code_idx ON language (code);
CREATE TABLE string (
    id SERIAL PRIMARY KEY,
    name varchar,
    domain_id integer REFERENCES domain(id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX domain_id_idx ON string (domain_id);
CREATE INDEX name_idx ON string (name);
CREATE UNIQUE INDEX name_domain_idx ON string (name, domain_id);
CREATE TABLE translation (
    id SERIAL PRIMARY KEY,
    language_id integer REFERENCES language(id) ON DELETE CASCADE ON UPDATE CASCADE,
    content TEXT,
    string_id integer REFERENCES string(id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX language_id_idx ON translation (language_id);
CREATE INDEX string_id_idx ON translation (string_id);
CREATE UNIQUE INDEX string_id_language_id_idx ON translation (language_id, string_id);
INSERT INTO language (name, code) VALUES
    ('German','de'),
    ('English','en'),
    ('Spanish','es'),
    ('French','fr'),
    ('Italian','it'),
    ('Polish','pl'),
    ('German (Austria)','de-at'),
    ('German (Switzerland)','de-ch'),
    ('German (Germany)','de-de'),
    ('English (Australia)','en-au'),
    ('English (Canada)','en-ca'),
    ('English (UK)','en-gb'),
    ('English (Bahrain)','en-bh'),
    ('English (US)','en-us'),
    ('English (South Africa)','en-za'),
    ('French (Canada)','fr-ca'),
    ('Portuguese','pt'),
    ('English (Ireland)','en-ie'),
    ('Czech','cs'),
    ('Hungarian','hu'),
    ('Spanish (US)','es-us');`,
		// 2
		`INSERT INTO language (code, name) VALUES ('nl', 'Duth');`,
	}
}

func (a PostgresAdapter) down() []string {
	return []string{
		// 1
		`
DROP TABLE IF EXISTS translation;
DROP TABLE IF EXISTS string;
DROP TABLE IF EXISTS language;
DROP TABLE IF EXISTS domain;
`,
		// 2
		`DELETE FROM language WHERE code = 'nl';`,
	}
}

func (a PostgresAdapter) MigrateUp(db *sqlx.DB) (version int64, err error) {
	startVer, err := a.version(db)
	if err != nil {
		return version, err
	}

	for i, query := range a.up() {
		migTo := int64(i + 1)
		if migTo <= startVer {
			version = migTo
			continue
		}

		_, err = db.Exec(query)
		if err != nil {
			return version, err
		}

		err = a.updateVersion(migTo, db)
		if err != nil {
			return version, err
		}

		version = migTo
	}

	return version, err
}

func (a PostgresAdapter) MigrateDown(db *sqlx.DB) (version int64, err error) {
	startVer, err := a.version(db)
	if err != nil {
		return version, err
	}

	down := a.down()
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

		err = a.updateVersion(migTo, db)
		if err != nil {
			return version, err
		}

		version = migTo
	}

	return version, err
}

func (a PostgresAdapter) SupportsLastInsertId() bool {
	return false
}

func (a PostgresAdapter) CreateDomainQuery() string {
	return `INSERT INTO domain (name) VALUES ($1) RETURNING id;`
}

func (a PostgresAdapter) CreateLanguageQuery() string {
	return `INSERT INTO language (code, name) VALUES ($1, $2) RETURNING id;`
}

func (a PostgresAdapter) CreateStringQuery() string {
	return `INSERT INTO string (name, domain_id) VALUES ($1, $2) RETURNING id;`
}

func (a PostgresAdapter) CreateTranslationQuery() string {
	return `INSERT INTO translation (language_id, content, string_id) VALUES ($1, $2, $3) RETURNING id;`
}

func (a PostgresAdapter) GetAllDomainsQuery() string {
	return `SELECT name FROM domain ORDER BY name;`
}

func (a PostgresAdapter) GetAllLanguagesQuery() string {
	return `SELECT id, code, name FROM language ORDER BY code;`
}

func (a PostgresAdapter) GetSingleDomainQuery() string {
	return `SELECT string.id AS string_id, string.name, translation.language_id AS language_id, language.code, translation.id AS translation_id, translation.content FROM string INNER JOIN translation ON string.id = translation.string_id INNER JOIN language ON translation.language_id = language.id WHERE string.domain_id = (SELECT id FROM domain where domain.name = $1) ORDER BY string.name;`
}

func (a PostgresAdapter) GetSingleDomainIdQuery() string {
	return `SELECT id FROM domain WHERE name=$1;`
}

func (a PostgresAdapter) GetSingleLanguageQuery() string {
	return `SELECT id, name, code FROM language WHERE code=$1;`
}

func (a PostgresAdapter) GetSingleStringIdQuery() string {
	return `SELECT id FROM string WHERE name = $1 AND domain_id = $2;`
}

func (a PostgresAdapter) GetSingleTranslationIdQuery() string {
	return `SELECT translation.id FROM string INNER JOIN translation ON string.id = translation.string_id WHERE string.id=$1 AND language_id=$2 AND domain_id=$3;`
}

func (a PostgresAdapter) UpdateTranslationQuery() string {
	return `UPDATE translation SET language_id=$1, content=$2, string_id=$3 WHERE id=$4;`
}

func (a PostgresAdapter) version(db *sqlx.DB) (version int64, err error) {
	row := db.QueryRow(`SELECT version FROM schema_migrations;`)
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

func (a PostgresAdapter) updateVersion(version int64, db *sqlx.DB) (err error) {
	_, err = db.Exec(`UPDATE schema_migrations SET version = $1`, int64(version))

	return err
}
