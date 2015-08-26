package datastore

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/petert82/go-translation-api/config"
	"github.com/petert82/go-translation-api/trans"
	"github.com/petert82/go-translation-api/xliff"
	"path/filepath"
	"time"
)

// Adapter provides database-driver-specific query strings, etc.
type Adapter interface {
	PostCreate(*sqlx.DB) error
	CreateDomainQuery() string
	CreateStringQuery() string
	CreateTranslationQuery() string
	GetAllDomainsQuery() string
	GetAllLanguagesQuery() string
	GetSingleDomainQuery() string
	GetSingleDomainIdQuery() string
	GetSingleLanguageQuery() string
	GetSingleStringIdQuery() string
	GetSingleTranslationIdQuery() string
	UpdateTranslationQuery() string
}

type DataStore struct {
	adapter     Adapter
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

// Creates a new datastore using the given database connection. The driver parameter is used to
// select the appropriate database adapter, and should be one of the config.DbDriver* constants.
func New(db *sqlx.DB, driver string) (ds *DataStore, err error) {
	adp, err := newAdapter(driver)
	if err != nil {
		return &DataStore{}, err
	}

	ds = &DataStore{
		adapter:     adp,
		db:          db,
		domainCache: make(map[string]int64),
		stringCache: make(map[StringKey]int64),
		Stats:       make(map[StatKey]StatItem),
	}

	err = ds.adapter.PostCreate(ds.db)
	if err != nil {
		return ds, err
	}

	return ds, nil
}

func newAdapter(driver string) (adp Adapter, err error) {
	switch driver {
	case config.DbDriverSqlite3:
		adp = &Sqlite3Adapter{}
	}

	if adp == nil {
		return nil, errors.New(fmt.Sprintf("no adapter available for database driver '%v'", driver))
	}

	return adp, nil
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

func (s String) Name() string {
	return s.name
}
func (s String) Translations() map[trans.Language]trans.Translation {
	return s.translations
}

type Translation struct {
	id      int64
	content string
}

func (t Translation) Content() string {
	return t.content
}

func (ds *DataStore) getLanguage(code string) (l trans.Language, err error) {
	start := time.Now()
	defer func() { ds.Stats.Log("language", "get", time.Since(start)) }()

	err = ds.db.Get(&l, ds.adapter.GetSingleLanguageQuery(), code)
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

	row := ds.db.QueryRow(ds.adapter.GetSingleDomainIdQuery(), name)
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

	result, err := ds.db.Exec(ds.adapter.CreateDomainQuery(), name)
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

	row := ds.db.QueryRow(ds.adapter.GetSingleStringIdQuery(), name, domainId)
	err = row.Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (ds *DataStore) createString(name string, domainId int64) (id int64, err error) {
	start := time.Now()
	defer func() { ds.Stats.Log("string", "insert", time.Since(start)) }()

	result, err := ds.db.Exec(ds.adapter.CreateStringQuery(), name, domainId)
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

	row := ds.db.QueryRow(ds.adapter.GetSingleTranslationIdQuery(), stringId, langId, domainId)
	err = row.Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (ds *DataStore) insertTranslation(t trans.Translation, langId int64, stringId int64, domainId int64) (err error) {
	start := time.Now()
	defer func() { ds.Stats.Log("translation", "insert", time.Since(start)) }()

	_, err = ds.db.Exec(ds.adapter.CreateTranslationQuery(), langId, t.Content(), stringId)

	return err
}

func (ds *DataStore) updateTranslation(t trans.Translation, transId int64, langId int64, stringId int64, domainId int64) (err error) {
	start := time.Now()
	defer func() { ds.Stats.Log("translation", "update", time.Since(start)) }()

	_, err = ds.db.Exec(ds.adapter.UpdateTranslationQuery(), langId, t.Content(), stringId, transId)

	return err
}

// Gets all available languages
func (ds *DataStore) GetLanguageList() (languages []trans.Language, err error) {
	start := time.Now()
	defer func() { ds.Stats.Log("language", "get", time.Since(start)) }()

	err = ds.db.Select(&languages, ds.adapter.GetAllLanguagesQuery())

	return languages, err
}

// Gets all available domains. Only populates name of each returned domain
func (ds *DataStore) GetDomainList() (domains []trans.Domain, err error) {
	start := time.Now()
	defer func() { ds.Stats.Log("domain", "get", time.Since(start)) }()

	rows, err := ds.db.Query(ds.adapter.GetAllDomainsQuery())
	if err != nil {
		return domains, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return domains, err
		}
		domains = append(domains, &Domain{name: name})
	}

	return domains, nil
}

// Gets all data for the translation domain with the given name.
// Returns sql.ErrNoRows when the given name cannot be found.
func (ds *DataStore) GetFullDomain(name string) (d trans.Domain, err error) {
	start := time.Now()
	defer func() { ds.Stats.Log("domain", "get", time.Since(start)) }()

	var rows []struct {
		StringId      int64  `db:"stringId"`
		Name          string `db:"name"`
		LanguageId    int64  `db:"languageId"`
		Code          string `db:"code"`
		TranslationId int64  `db:"translationId"`
		Content       string `db:"content"`
	}
	err = ds.db.Select(&rows, ds.adapter.GetSingleDomainQuery(), name)
	if err != nil {
		return d, err
	}

	if len(rows) == 0 {
		return d, sql.ErrNoRows
	}

	dom := Domain{name: name, strings: make([]trans.String, 0)}
	stringIndex := make(map[string]int64)
	var i int64 = 0

	for _, r := range rows {
		l := trans.Language{Id: r.LanguageId, Code: r.Code}
		t := Translation{id: r.TranslationId, content: r.Content}

		if sIdx, ok := stringIndex[r.Name]; ok == true {
			dom.strings[sIdx].(*String).translations[l] = &t
		} else {
			s := &String{id: r.StringId, name: r.Name, translations: make(map[trans.Language]trans.Translation)}
			s.translations[l] = &t
			dom.strings = append(dom.strings, s)
			stringIndex[r.Name] = i
			i++
		}
	}

	return &dom, nil
}

// Updates the translation of the string with the given name to have the given content.
// When allowCreate is false, will return an error if the string does not exist or is not yet
// translated into the given language.
// If allowCreate is true, both the string and translation content for the given language will be
// created if either does not exist.
func (ds *DataStore) CreateOrUpdateTranslation(domainName, stringName, langCode, content string, allowCreate bool) (err error) {
	domId, err := ds.getDomainId(domainName)
	if err != nil {
		return err
	}

	var stringId int64
	if allowCreate {
		stringId, err = ds.createOrGetString(stringName, domId)
	} else {
		stringId, err = ds.getStringId(stringName, domId)
	}
	if err != nil {
		return err
	}

	lang, err := ds.getLanguage(langCode)
	if err != nil {
		return err
	}

	t := &Translation{content: content}
	transId, err := ds.getTranslationId(t, lang.Id, stringId, domId)
	if err != nil && !allowCreate {
		return err
	} else if err == sql.ErrNoRows && allowCreate {
		err = ds.insertTranslation(t, lang.Id, stringId, domId)
	} else if err == nil {
		err = ds.updateTranslation(t, transId, lang.Id, stringId, domId)
	}

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

func (ds *DataStore) ExportDomain(name, dir string) (err error) {
	d, err := ds.GetFullDomain(name)
	if err != nil {
		return err
	}

	return xliff.Export(d, trans.Language{Id: 7, Code: "en", Name: "English"}, dir)
}
