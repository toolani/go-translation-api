package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/petert82/go-translation-api/config"
	"github.com/petert82/go-translation-api/datastore"
	"net/http"
	"os"
)

var (
	export    chan string
	exportDir string
)

func checkFatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func checkHttpWithStatus(e error, w http.ResponseWriter, status int) (hadError bool) {
	if e != nil {
		w.WriteHeader(status)

		errMsg := e.Error()
		// Don't expose the 'sql: no rows in result set' message to the user
		if status == http.StatusNotFound && e == sql.ErrNoRows {
			errMsg = "not found"
		}

		jsonErr := struct {
			Error string `json:"error"`
		}{
			Error: errMsg,
		}
		enc := json.NewEncoder(w)
		enc.Encode(jsonErr)

		return true
	}
	return false
}

func checkHttp(e error, w http.ResponseWriter) (hadError bool) {
	status := http.StatusInternalServerError
	if e == sql.ErrNoRows {
		status = http.StatusNotFound
	}
	return checkHttpWithStatus(e, w, status)
}

// Instantiates a datastore for a request using the given DB connection
func handleWithDatastore(db *sqlx.DB, driver string, f func(http.ResponseWriter, *http.Request, *datastore.DataStore)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds, err := datastore.New(db, driver)

		if checkHttpWithStatus(err, w, http.StatusServiceUnavailable) {
			return
		}
		f(w, r, ds)
	}
}

func setJsonHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		h.ServeHTTP(w, r)
	})
}

// Gets list of available languages
func getLanguagesHandler(w http.ResponseWriter, r *http.Request, ds *datastore.DataStore) {
	ls, err := ds.GetLanguageList()
	if checkHttp(err, w) {
		return
	}

	enc := json.NewEncoder(w)
	checkHttp(enc.Encode(ls), w)
}

// Gets list of available translation domain names
func getDomainsHandler(w http.ResponseWriter, r *http.Request, ds *datastore.DataStore) {
	doms, err := ds.GetDomainList()
	if checkHttp(err, w) {
		return
	}

	var output struct {
		Domains []string `json:"domains"`
	}
	output.Domains = make([]string, len(doms))
	for i, d := range doms {
		output.Domains[i] = d.Name()
	}

	enc := json.NewEncoder(w)
	checkHttp(enc.Encode(output), w)
}

// Get a domain and all its strings & translations
func getDomainHandler(w http.ResponseWriter, r *http.Request, ds *datastore.DataStore) {
	name := mux.Vars(r)["name"]

	dom, err := ds.GetFullDomain(name)
	if checkHttp(err, w) {
		return
	}

	enc := json.NewEncoder(w)
	checkHttp(enc.Encode(NewDomain(dom)), w)
}

// Export a domain to XLIFF files on disk
func exportDomainHandler(w http.ResponseWriter, r *http.Request, ds *datastore.DataStore) {
	name := mux.Vars(r)["name"]

	err := ds.ExportDomain(name, exportDir)
	if checkHttp(err, w) {
		return
	}

	w.Write([]byte("{\"result\":\"ok\"}\n"))
}

// Update a translation with new content (or create it if we have a POST request)
func createOrUpdateTranslationHandler(w http.ResponseWriter, r *http.Request, ds *datastore.DataStore) {
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
	if checkHttp(err, w) {
		return
	}

	w.Write([]byte("{\"result\":\"ok\"}\n"))

	export <- dName
}

func Serve(c config.Config) {
	exportDir = c.XLIFF.ExportPath
	export = make(chan string, 100)

	var db *sqlx.DB
	db, err := sqlx.Connect(c.DB.Driver, c.DB.ConnectionString())
	checkFatal(err)

	// Listen for domains to export to file
	go func() {
		ds, err := datastore.New(db, c.DB.Driver)
		checkFatal(err)

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
	languages.Methods("GET").HandlerFunc(handleWithDatastore(db, c.DB.Driver, getLanguagesHandler))

	domains := r.Path("/domains").Subrouter()
	domains.Methods("GET").HandlerFunc(handleWithDatastore(db, c.DB.Driver, getDomainsHandler))

	domain := r.PathPrefix("/domains/{name}").Subrouter()
	domain.Methods("GET").HandlerFunc(handleWithDatastore(db, c.DB.Driver, getDomainHandler))
	domain.Methods("POST").Path("/export").HandlerFunc(handleWithDatastore(db, c.DB.Driver, exportDomainHandler))

	translation := r.PathPrefix("/domains/{domain}/strings/{string}/translations/{lang}")
	translation.Methods("POST", "PUT").HandlerFunc(handleWithDatastore(db, c.DB.Driver, createOrUpdateTranslationHandler))

	rWithMiddleWares := handlers.CombinedLoggingHandler(os.Stdout, setJsonHeaders(r))

	fmt.Printf("Listening on port %v\n", c.Server.Port)
	http.ListenAndServe(fmt.Sprintf(":%v", c.Server.Port), rWithMiddleWares)
}
