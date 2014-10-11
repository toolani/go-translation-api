package datastore

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/petert82/go-translation-api/trans"
	"github.com/petert82/go-translation-api/xliff"
	"path/filepath"
)

type DataStore struct {
	db          *sqlx.DB
	domainCache map[string]int64
	stringCache map[StringKey]int64
}

type StringKey struct {
	DomainId int64
	Name     string
}

func New(db *sqlx.DB) *DataStore {
	return &DataStore{db: db, domainCache: make(map[string]int64), stringCache: make(map[StringKey]int64)}
}

func (ds *DataStore) getLanguage(code string) (l trans.Language, err error) {
	err = ds.db.Get(&l, "SELECT id, name, code FROM language WHERE code=?", code)
	if err != nil {
		if err == sql.ErrNoRows {
			return l, errors.New(fmt.Sprintf("Language '%v' does not exist in database", code))
		}

		return l, err
	}

	return l, nil
}

func (ds *DataStore) getDomainId(name string) (id int64, err error) {
	if id, ok := ds.domainCache[name]; ok {
		return id, nil
	}

	row := ds.db.QueryRow("SELECT id FROM domain WHERE name=? ", name)
	err = row.Scan(&id)
	if err != nil {
		return 0, err
	}
	ds.domainCache[name] = id

	return id, nil
}

func (ds *DataStore) createDomain(name string) (id int64, err error) {
	result, err := ds.db.Exec("INSERT INTO domain (name) VALUES (?)", name)
	if err != nil {
		return 0, nil
	}

	id, err = result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (ds *DataStore) createOrGetDomain(name string) (id int64, err error) {
	id, err = ds.getDomainId(name)

	if err == sql.ErrNoRows {
		return ds.createDomain(name)
	}

	return id, err
}

func (ds *DataStore) getTranslationId(t *trans.Translation, domainId int64) (id int, err error) {
	row := ds.db.QueryRow("SELECT translation.id FROM string INNER JOIN translation ON string.id = translation.string_id WHERE name=? AND language_id=? AND domain_id=?", t.Name, t.Language.Id, domainId)
	err = row.Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (ds *DataStore) upsertString(name string, domainId int64) (id int64, err error) {
	result, err := ds.db.Exec(`INSERT OR REPLACE INTO string (id, name, domain_id) VALUES ((SELECT id FROM string WHERE name = ? AND domain_id = ?), ?, ?)`, name, domainId, name, domainId)
	if err != nil {
		return 0, err
	}

	id, err = result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (ds *DataStore) insertTranslation(t *trans.Translation, stringId int64, domainId int64) (err error) {
	_, err = ds.db.Exec(`INSERT INTO translation (language_id, content, string_id) VALUES (?, ?, ?)`, t.Language.Id, t.Content, stringId)
	return err
}

func (ds *DataStore) updateTranslation(t *trans.Translation, stringId int64, domainId int64) (err error) {
	_, err = ds.db.Exec(`UPDATE translation SET language_id=?, content=?, string_id=? WHERE id=?`, t.Language.Id, t.Content, stringId, t.Id)
	return err
}

func (ds *DataStore) ImportDomain(d trans.Domain) (err error) {
	l, err := ds.getLanguage(d.Language())
	if err != nil {
		return err
	}

	domId, err := ds.createOrGetDomain(d.Name())
	if err != nil {
		return err
	}

	for _, t := range d.Translations() {
		t.Language = &l

		stringId, ok := ds.stringCache[StringKey{DomainId: domId, Name: t.Name}]
		if !ok {
			stringId, err = ds.upsertString(t.Name, domId)
			if err != nil {
				return err
			}
			ds.stringCache[StringKey{DomainId: domId, Name: t.Name}] = stringId
		}

		if t.Id, err = ds.getTranslationId(&t, domId); err != nil {
			if err == sql.ErrNoRows {
				err = ds.insertTranslation(&t, stringId, domId)
			}
		} else {
			err = ds.updateTranslation(&t, stringId, domId)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (ds *DataStore) ImportDir(dir string, notify chan string) (count int, err error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.xliff"))
	if err != nil {
		return 0, nil
	}

	for i, file := range files {
		xliff, err := xliff.NewFromFile(file)
		if err != nil {
			return i, err
		}

		err = ds.ImportDomain(&xliff.File.XliffDomain)
		if err != nil {
			return i, err
		}

		notify <- filepath.Base(file)
	}

	return len(files), nil
}
