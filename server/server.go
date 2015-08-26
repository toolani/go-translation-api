package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/petert82/go-translation-api/config"
	"github.com/petert82/go-translation-api/datastore"
	"net/http"
	"os"
)

var (
	ds        *datastore.DataStore
	export    chan string
	exportDir string
)

func init() {
	export = make(chan string, 100)
}

func checkFatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func checkHttpWithStatus(e error, w http.ResponseWriter, status int) (hadError bool) {
	if e != nil {
		http.Error(w, fmt.Sprintf("Error: %v", e.Error()), status)
		return true
	}
	return false
}

func checkHttp(e error, w http.ResponseWriter) (hadError bool) {
	return checkHttpWithStatus(e, w, http.StatusInternalServerError)
}

func parseArgs(args []string) (dbPath string, exportDir string, err error) {
	if len(args) < 2 {
		return "", "", errors.New("Usage:\n  transserver [-p <port>] DB_PATH EXPORT_PATH")
	}

	return args[0], args[1], nil
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

	err := ds.ExportDomain(name, exportDir)
	if checkHttp(err, w) {
		return
	}

	w.Write([]byte("{\"result\":\"ok\"}\n"))
}

// Update a translation with new content (or create it if we have a POST request)
func createOrUpdateTranslationHandler(w http.ResponseWriter, r *http.Request) {
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

	allowCreate := false
	if r.Method == "POST" {
		allowCreate = true
	}

	err = ds.CreateOrUpdateTranslation(dName, sName, lang, content.Content, allowCreate)
	status := http.StatusInternalServerError
	if err == sql.ErrNoRows {
		status = http.StatusNotFound
	}
	if checkHttpWithStatus(err, w, status) {
		return
	}

	w.Write([]byte("{\"result\":\"ok\"}\n"))

	export <- dName
}

func Serve(c config.Config) {
	var db *sqlx.DB
	db, err := sqlx.Connect(c.DB.Driver, c.DB.ConnectionString())
	checkFatal(err)
	ds, err = datastore.New(db)
	checkFatal(err)

	// Listen for domains to export to file
	go func() {
		for {
			d := <-export
			err := ds.ExportDomain(d, c.XLIFF.ExportPath)
			if err != nil {
				fmt.Println(err)
			}
		}
	}()

	r := mux.NewRouter().StrictSlash(true)

	languages := r.Path("/languages").Subrouter()
	languages.Methods("GET").HandlerFunc(getLanguagesHandler)

	domains := r.Path("/domains").Subrouter()
	domains.Methods("GET").HandlerFunc(getDomainsHandler)

	domain := r.PathPrefix("/domains/{name}").Subrouter()
	domain.Methods("GET").HandlerFunc(getDomainHandler)
	domain.Methods("POST").Path("/export").HandlerFunc(exportDomainHandler)

	translation := r.PathPrefix("/domains/{domain}/strings/{string}/translations/{lang}")
	translation.Methods("POST", "PUT").HandlerFunc(createOrUpdateTranslationHandler)

	rWithMiddleWares := handlers.CombinedLoggingHandler(os.Stdout, setJsonHeaders(r))

	fmt.Printf("Listening on port %v\n", c.Server.Port)
	http.ListenAndServe(fmt.Sprintf(":%v", c.Server.Port), rWithMiddleWares)
}
