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
    ('Spanish (US)','es-us'),
    ('Dutch', 'nl'),
    ('English (NL)', 'en-nl'),
    ('Dutch (BE)', 'nl-be'),
    ('English (CH)', 'en-ch'),
    ('Spanish (AR)', 'es-ar'),
    ('Spanish (CL)', 'es-cl'),
    ('Spanish (MX)', 'es-mx'),
    ('Spanish (PE)', 'es-pe'),
    ('French (CH)', 'fr-ch'),
    ('Spanish (CO)', 'es-co'),
    ('English (BE)', 'en-be'),
    ('English (CZ)', 'en-cz'),
    ('English (HU)', 'en-hu'),
    ('English (PL)', 'en-pl'),
    ('French (BE)', 'fr-be'),
    ('Italian (CH)', 'it-ch'),
    ('English (AT)', 'en-at'),
    ('English (ES)', 'en-es'),
    ('English (FR)', 'en-fr'),
    ('English (IT)', 'en-it'),
    ('German (BE)', 'de-be'),
    ('German (ES)', 'de-es'),
    ('English (AR)', 'en-ar'),
    ('English (CL)', 'en-cl'),
    ('English (CO)', 'en-co'),
    ('English (DE)', 'en-de'),
    ('English (MX)', 'en-mx'),
    ('English (PE)', 'en-pe');`,
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
		// 3
		`UPDATE language SET name = 'Duth' WHERE name = 'Dutch' AND code = 'nl';`,
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

func (a PostgresAdapter) DeleteStringQuery() string {
	return `DELETE FROM string WHERE id = $1;`
}

func (a PostgresAdapter) DeleteTranslationQuery() string {
	return `DELETE FROM translation WHERE id = $1;`
}

func (a PostgresAdapter) GetAllDomainsQuery() string {
	return `SELECT name FROM domain ORDER BY name;`
}

func (a PostgresAdapter) GetAllLanguagesQuery() string {
	return `SELECT id, code, name FROM language ORDER BY code;`
}

func (a PostgresAdapter) GetSearchByStringNameQuery() string {
	return `
SELECT
    d.id AS domain_id,
    d.name AS domain_name,
    s.id AS string_id,
    s.name AS string_name,
    l.id AS language_id,
    l.code AS language_code,
    t.id AS translation_id,
    t.content AS translation_content
FROM translation t
INNER JOIN string s ON s.id = t.string_id
INNER JOIN language l ON t.language_id = l.id
INNER JOIN domain d ON s.domain_id = d.id
WHERE s.name LIKE $1
LIMIT 100;`
}

func (a PostgresAdapter) GetSearchByTranslationContentQuery() string {
	return `
SELECT
    d.id AS domain_id,
    d.name AS domain_name,
    s.id AS string_id,
    s.name AS string_name,
    l.id AS language_id,
    l.code AS language_code,
    t.id AS translation_id,
    t.content AS translation_content
FROM translation t
INNER JOIN string s ON s.id = t.string_id
INNER JOIN language l ON t.language_id = l.id
INNER JOIN domain d ON s.domain_id = d.id
WHERE t.content LIKE $1
LIMIT 100;`
}

func (a PostgresAdapter) GetSearchByAllFieldsQuery() string {
	return `
SELECT
    d.id AS domain_id,
    d.name AS domain_name,
    s.id AS string_id,
    s.name AS string_name,
    l.id AS language_id,
    l.code AS language_code,
    t.id AS translation_id,
    t.content AS translation_content
FROM translation t
INNER JOIN string s ON s.id = t.string_id
INNER JOIN language l ON t.language_id = l.id
INNER JOIN domain d ON s.domain_id = d.id
WHERE s.name LIKE $1 OR t.content LIKE $2
LIMIT 100;`
}

func (a PostgresAdapter) GetSingleDomainQuery() string {
	return `
SELECT 
    d.id AS domain_id,
    s.id AS string_id,
    s.name AS string_name,
    t.language_id AS language_id,
    l.code AS language_code,
    t.id AS translation_id,
    t.content
FROM domain d
LEFT JOIN string s ON d.id = s.domain_id
LEFT JOIN translation t ON s.id = t.string_id 
LEFT JOIN language l ON t.language_id = l.id 
WHERE d.name = $1
ORDER BY s.name;`
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
