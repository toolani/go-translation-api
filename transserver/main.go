package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/petert82/go-translation-api/datastore"
	"github.com/petert82/go-translation-api/trans"
	"github.com/stephens2424/muxchain"
	"github.com/stephens2424/muxchain/muxchainutil"
	"net/http"
	"os"
)

type Domain struct {
	Name    string   `json:"name"`
	Strings []String `json:"strings"`
}

func NewDomain(dd trans.Domain) (d *Domain) {
	ds := dd.Strings()
	d = &Domain{Name: dd.Name(), Strings: make([]String, len(ds))}

	for i, s := range ds {
		ns := String{Name: s.Name(), Translations: make(map[string]Translation)}
		for l, t := range s.Translations() {
			ns.Translations[l.Code] = Translation{Content: t.Content()}
		}
		d.Strings[i] = ns
	}

	return d
}

type String struct {
	Name         string                 `json:"name"`
	Translations map[string]Translation `json:"translations"`
}

type Translation struct {
	Content string `json:"content"`
}

var (
	ds   *datastore.DataStore
	port int
)

func init() {
	flag.IntVar(&port, "p", 8181, "Port to start server on")
}

func check(e error) {
	if e != nil {
		fmt.Println(e)
		os.Exit(1)
	}
}

func parseArgs(args []string) (dbPath string, err error) {
	if len(args) < 1 {
		return "", errors.New("Usage:\n  transimporter [-p <port>] DB_PATH IMPORT_PATH")
	}

	return args[0], nil
}

func setJsonHeaders(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
}

func getDomain(w http.ResponseWriter, req *http.Request) {
	name := req.FormValue("name")
	if name == "" {
		http.Error(w, "No name given", http.StatusNotFound)
		return
	}

	dom, err := ds.GetFullDomain(name)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: %v", err.Error()), http.StatusBadRequest)
		return
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(NewDomain(dom)); err != nil {
		http.Error(w, fmt.Sprintf("Error: %v", err.Error()), http.StatusBadRequest)
		return
	}
}

func updateTranslation(w http.ResponseWriter, req *http.Request) {
	dName := req.FormValue("domain")
	if dName == "" {
		http.Error(w, "No domain given", http.StatusNotFound)
		return
	}

	sName := req.FormValue("string")
	if sName == "" {
		http.Error(w, "No string given", http.StatusNotFound)
		return
	}

	lang := req.FormValue("lang")
	if lang == "" {
		http.Error(w, "No lang given", http.StatusNotFound)
		return
	}

	var content struct {
		Content string `json:"content"`
	}

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&content)
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not decode request (%v)", err.Error()), http.StatusBadRequest)
		return
	}

	err = ds.UpdateTranslation(dName, sName, lang, content.Content)
	if err != nil {
		http.Error(w, "Error: %v", http.StatusInternalServerError)
		return
	}
	w.Write([]byte("{\"result\":\"ok\"}\n"))
}

func main() {
	flag.Parse()
	dbFile, err := parseArgs(flag.Args())
	check(err)

	var db *sqlx.DB
	db, err = sqlx.Connect("sqlite3", dbFile)
	check(err)
	ds, err = datastore.New(db)
	check(err)

	headerHandler := http.HandlerFunc(setJsonHeaders)

	pathHandler := muxchainutil.NewPathMux()
	pathHandler.Handle("/domains/:name", http.HandlerFunc(getDomain))
	pathHandler.Handle("/domains/:domain/strings/:string/translations/:lang", http.HandlerFunc(updateTranslation))

	muxchain.Chain("/", headerHandler, pathHandler)
	http.ListenAndServe(fmt.Sprintf(":%v", port), muxchain.Default)
}
