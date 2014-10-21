package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/petert82/go-translation-api/datastore"
	"github.com/petert82/go-translation-api/trans"
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

func checkHttp(e error, w http.ResponseWriter) (hadError bool) {
	if e != nil {
		http.Error(w, fmt.Sprintf("Error: %v", e.Error()), http.StatusInternalServerError)
		return true
	}
	return false
}

func parseArgs(args []string) (dbPath string, err error) {
	if len(args) < 1 {
		return "", errors.New("Usage:\n  transserver [-p <port>] DB_PATH")
	}

	return args[0], nil
}

func setJsonHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		h.ServeHTTP(w, r)
	})
}

// Gets list of available languages
func getLanguagesHandler(w http.ResponseWriter, r *http.Request) {
	ls, err := ds.GetLanguageList()
	if checkHttp(err, w) {
		return
	}

	enc := json.NewEncoder(w)
	checkHttp(enc.Encode(ls), w)
}

// Gets list of available translation domain names
func getDomainsHandler(w http.ResponseWriter, r *http.Request) {
	ds, err := ds.GetDomainList()
	if checkHttp(err, w) {
		return
	}

	var output struct {
		Domains []string `json:"domains"`
	}
	output.Domains = make([]string, len(ds))
	for i, d := range ds {
		output.Domains[i] = d.Name()
	}

	enc := json.NewEncoder(w)
	checkHttp(enc.Encode(output), w)
}

// Get a domain and all its strings & translations
func getDomainHandler(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]

	dom, err := ds.GetFullDomain(name)
	if checkHttp(err, w) {
		return
	}

	enc := json.NewEncoder(w)
	checkHttp(enc.Encode(NewDomain(dom)), w)
}

// Export a domain to XLIFF files on disk
func exportDomainHandler(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]

	err := ds.ExportDomain(name, "/Users/pthompson/temp/xliff")
	if checkHttp(err, w) {
		return
	}

	w.Write([]byte("{\"result\":\"ok\"}\n"))
}

// Update a translation with new content
func updateTranslationHandler(w http.ResponseWriter, r *http.Request) {
	dName := mux.Vars(r)["domain"]
	sName := mux.Vars(r)["string"]
	lang := mux.Vars(r)["lang"]

	var content struct {
		Content string `json:"content"`
	}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&content)
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not decode request (%v)", err.Error()), http.StatusBadRequest)
		return
	}

	err = ds.UpdateTranslation(dName, sName, lang, content.Content)
	if checkHttp(err, w) {
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

	r := mux.NewRouter().StrictSlash(true)

	languages := r.Path("/languages").Subrouter()
	languages.Methods("GET").HandlerFunc(getLanguagesHandler)

	domains := r.Path("/domains").Subrouter()
	domains.Methods("GET").HandlerFunc(getDomainsHandler)

	domain := r.PathPrefix("/domains/{name}").Subrouter()
	domain.Methods("GET").HandlerFunc(getDomainHandler)
	domain.Methods("POST").Path("/export").HandlerFunc(exportDomainHandler)

	translation := r.PathPrefix("/domains/{domain}/strings/{string}/translations/{lang}")
	translation.Methods("PUT").HandlerFunc(updateTranslationHandler)

	rWithMiddleWares := handlers.CombinedLoggingHandler(os.Stdout, setJsonHeaders(r))

	fmt.Printf("Listening on port %v\n", port)
	http.ListenAndServe(fmt.Sprintf(":%v", port), rWithMiddleWares)
}
