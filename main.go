package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	con "github.com/dennyaris/html-rotate/adapter"
	con_api "github.com/dennyaris/html-rotate/adapter/api"
	"github.com/dennyaris/html-rotate/util"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

// DB Config
const (
	DBUser = "root"
	DBPass = ""
	DBHost = "localhost"
	DBPort = "3306"
	DBName = "builder"
)

var db *sql.DB

func connectDatabase() (*sql.DB, error) {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", DBUser, DBPass, DBHost, DBPort, DBName))
	if err != nil {
		return nil, err
	}
	return db, nil
}

func disconnectDatabase(*sql.DB) {
	if db != nil {
		db.Close()
	}
}

// memcached config
const (
	MemcachedHost = "localhost"
	MemcachedPort = "11211"
)

func main() {
	var err error
	db, err = connectDatabase() // Connect to the database
	if err != nil {
		fmt.Println("Error connecting to the database:", err)
		os.Exit(1)
	}
	defer disconnectDatabase(db) // Ensure the database connection is closed when main() exits

	util.InitMemcached(fmt.Sprintf("%s:%s", MemcachedHost, MemcachedPort))

	route := mux.NewRouter()
	route.HandleFunc("/rotate", func(w http.ResponseWriter, r *http.Request) {
		err := con.RotateHandler(w, r, db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}).Methods("GET")
	route.HandleFunc("/flushall", func(w http.ResponseWriter, r *http.Request) {
		util.Flush()
		util.ResponseSuccess(w, nil, "success flush memcached")
	}).Methods("GET")

	// API
	apiHandler := con_api.Handler{
		DB: db,
	}
	route.HandleFunc("/api/page/create", apiHandler.CreatePage).Methods("POST")
	route.HandleFunc("/api/page/{id}", apiHandler.GetPage).Methods("GET")
	route.HandleFunc("/api/page/update/{id}", apiHandler.Update).Methods("PATCH")
	route.HandleFunc("/api/page/delete/{id}", apiHandler.DeletePage).Methods("DELETE")
	route.HandleFunc("/api/memcached/update/{key}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		key := vars["key"]

		if key == "" {
			util.ResponseError(w, "params key is empty", http.StatusBadRequest)
			return
		}

		var newData con.PageData
		if err := json.NewDecoder(r.Body).Decode(&newData); err != nil {
			util.ResponseError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_, err = util.GetMemcachedValue(key)
		if err != nil {
			util.ResponseError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		jsonData, _ := json.Marshal(newData)
		dataCache, err := util.UpdateValueMemcached(key, jsonData, 60)
		if err != nil {
			util.ResponseError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		util.ResponseSuccess(w, string(dataCache.Value), "success update memcached")
	}).Methods("PATCH")

	http.ListenAndServe(":9090", route)
}
