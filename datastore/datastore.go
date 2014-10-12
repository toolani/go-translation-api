package datastore

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/petert82/go-translation-api/trans"
	"github.com/petert82/go-translation-api/xliff"
	"path/filepath"
	"time"
)

type DataStore struct {
	db          *sqlx.DB
	domainCache map[string]int64
	stringCache map[StringKey]int64
	Stats       Stats
}

type StringKey struct {
	DomainId int64
	Name     string
}

type Stats map[StatKey]StatItem

type StatKey struct {
	Name   string
	Action string
}

type StatItem struct {
	Duration time.Duration
	Count    int
}

func (s Stats) Log(name, action string, d time.Duration) {
	item := s[StatKey{Name: name, Action: action}]
	item.Count++
	item.Duration += d
	s[StatKey{Name: name, Action: action}] = item
}

func (s Stats) String() (out string) {
	for k, v := range s {
		out += fmt.Sprintf("%v  %v '%v' actions took %v total, %v avg\n", v.Count, k.Name, k.Action, v.Duration, v.Duration/time.Duration(v.Count))
	}

	return out
}

func New(db *sqlx.DB) (ds *DataStore, err error) {
	ds = &DataStore{
		db:          db,
		domainCache: make(map[string]int64),
		stringCache: make(map[StringKey]int64),
		Stats:       make(map[StatKey]StatItem),
	}

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		return ds, err
	}
	// Faster than using default journal file
	_, err = db.Exec("PRAGMA journal_mode = WAL")
	if err != nil {
		return ds, err
	}
	// Default (full) is slower
	_, err = db.Exec("PRAGMA synchronous = NORMAL")
	if err != nil {
		return ds, err
	}
	return ds, nil
}

type Domain struct {
	name    string
	strings []trans.String
}

func (d *Domain) Name() string {
	return d.name
}
func (d *Domain) SetName(name string) {
	d.name = name
}
func (d *Domain) Strings() []trans.String {
	return d.strings
}

type String struct {
	id           int64
	name         string
	translations map[trans.Language]trans.Translation
}

func (s *String) Name() string {
	return s.name
}
func (s *String) SetName(name string) {
	s.name = name
}
func (s *String) Translations() map[trans.Language]trans.Translation {
	return s.translations
}

type Translation struct {
	id      int64
	content string
}

func (t *Translation) Content() string {
	return t.content
}
func (t *Translation) SetContent(content string) {
	t.content = content
}

func (ds *DataStore) getLanguage(code string) (l trans.Language, err error) {
	start := time.Now()
	defer func() { ds.Stats.Log("language", "get", time.Since(start)) }()

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
	start := time.Now()
	defer func() { ds.Stats.Log("domain", "get", time.Since(start)) }()

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
	start := time.Now()
	defer func() { ds.Stats.Log("domain", "insert", time.Since(start)) }()

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

func (ds *DataStore) getStringId(name string, domainId int64) (id int64, err error) {
	start := time.Now()
	defer func() { ds.Stats.Log("string", "get", time.Since(start)) }()

	row := ds.db.QueryRow("SELECT id FROM string WHERE name = ? AND domain_id = ?", name, domainId)
	err = row.Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (ds *DataStore) createString(name string, domainId int64) (id int64, err error) {
	start := time.Now()
	defer func() { ds.Stats.Log("string", "insert", time.Since(start)) }()

	result, err := ds.db.Exec(`INSERT INTO string (name, domain_id) VALUES (?, ?)`, name, domainId)
	if err != nil {
		return 0, err
	}

	id, err = result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (ds *DataStore) createOrGetString(name string, domainId int64) (id int64, err error) {
	id, err = ds.getStringId(name, domainId)

	if err == sql.ErrNoRows {
		id, err = ds.createString(name, domainId)
	}

	return id, err
}

func (ds *DataStore) getTranslationId(t trans.Translation, langId int64, stringId int64, domainId int64) (id int64, err error) {
	start := time.Now()
	defer func() { ds.Stats.Log("translation", "get", time.Since(start)) }()

	row := ds.db.QueryRow("SELECT translation.id FROM string INNER JOIN translation ON string.id = translation.string_id WHERE string.id=? AND language_id=? AND domain_id=?", stringId, langId, domainId)
	err = row.Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (ds *DataStore) insertTranslation(t trans.Translation, langId int64, stringId int64, domainId int64) (err error) {
	start := time.Now()
	defer func() { ds.Stats.Log("translation", "insert", time.Since(start)) }()

	_, err = ds.db.Exec(`INSERT INTO translation (language_id, content, string_id) VALUES (?, ?, ?)`, langId, t.Content(), stringId)

	return err
}

func (ds *DataStore) updateTranslation(t trans.Translation, transId int64, langId int64, stringId int64, domainId int64) (err error) {
	start := time.Now()
	defer func() { ds.Stats.Log("translation", "update", time.Since(start)) }()

	_, err = ds.db.Exec(`UPDATE translation SET language_id=?, content=?, string_id=? WHERE id=?`, langId, t.Content(), stringId, transId)

	return err
}

func (ds *DataStore) GetFullDomain(name string) (d trans.Domain, err error) {
	var rows []struct {
		StringId      int64  `db:"stringId"`
		Name          string `db:"name"`
		LanguageId    int64  `db:"languageId"`
		Code          string `db:"code"`
		TranslationId int64  `db:"translationId"`
		Content       string `db:"content"`
	}
	err = ds.db.Select(&rows, "SELECT string.id AS stringId, string.name, translation.language_id AS languageId, language.code, translation.id AS translationId, translation.content FROM string INNER JOIN translation ON string.id = translation.string_id INNER JOIN language ON translation.language_id = language.id WHERE string.domain_id = (SELECT id FROM domain where domain.name = ?)", name)
	if err != nil {
		return d, err
	}

	dom := Domain{name: "", strings: make([]trans.String, 0)}
	strings := make(map[string]String)

	for _, r := range rows {
		if dom.name == "" {
			dom.SetName(r.Name)
		}
		l := trans.Language{Id: r.LanguageId, Code: r.Code}
		t := Translation{id: r.TranslationId, content: r.Content}
		if s, ok := strings[r.Name]; ok == true {
			s.translations[l] = &t
		} else {
			s := String{id: r.StringId, name: r.Name, translations: make(map[trans.Language]trans.Translation)}
			s.translations[l] = &t
			strings[r.Name] = s
		}
	}

	for _, s := range strings {
		dom.strings = append(dom.strings, &String{id: s.id, name: s.name, translations: s.translations})
	}

	return &dom, nil
}

func (ds *DataStore) UpdateTranslation(domainName, stringName, langCode, content string) (err error) {
	domId, err := ds.getDomainId(domainName)
	if err != nil {
		return err
	}

	stringId, err := ds.getStringId(stringName, domId)
	if err != nil {
		return err
	}

	lang, err := ds.getLanguage(langCode)
	if err != nil {
		return err
	}

	t := &Translation{content: content}
	transId, err := ds.getTranslationId(t, lang.Id, stringId, domId)
	if err != nil {
		return err
	}

	err = ds.updateTranslation(t, transId, lang.Id, stringId, domId)

	return err
}

func (ds *DataStore) ImportDomain(d trans.Domain) (err error) {

	domId, err := ds.createOrGetDomain(d.Name())
	if err != nil {
		return err
	}

	for _, s := range d.Strings() {
		// Get the string's ID
		stringId, ok := ds.stringCache[StringKey{DomainId: domId, Name: s.Name()}]
		if !ok {
			stringId, err = ds.createOrGetString(s.Name(), domId)
			if err != nil {
				return err
			}
			ds.stringCache[StringKey{DomainId: domId, Name: s.Name()}] = stringId
		}

		for l, t := range s.Translations() {
			lang, err := ds.getLanguage(l.Code)
			if err != nil {
				return err
			}

			if transId, err := ds.getTranslationId(t, lang.Id, stringId, domId); err == nil {
				err = ds.updateTranslation(t, transId, lang.Id, stringId, domId)
			} else {
				if err == sql.ErrNoRows {
					err = ds.insertTranslation(t, lang.Id, stringId, domId)
				}
			}

			if err != nil {
				return err
			}
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
