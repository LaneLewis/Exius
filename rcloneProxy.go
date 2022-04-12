package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/lanelewis/rclone-proxy/database"
	"github.com/lanelewis/rclone-proxy/handles"
	"github.com/rs/cors"

	"github.com/gorilla/mux"
)

//todo: change all to unsigned int
// add ability for multiple rclone endpoints. Use inside endpoin and append to prefix
func main() {
	adminKey := os.Getenv("ADMINKEY")
	url := os.Getenv("DATABASE_URL") //"postgres://postgres:postgres@db:5432/postgres"
	err := database.DestroyDB(url)
	db, err := database.BuildDB(url)
	database.ClearExpiredKeys(db)
	if err != nil {
		log.Fatal(err)
	}
	//err = database.DeleteKey("1234", db)
	err = database.AddAdmin(adminKey, db)
	if err != nil {
		if fmt.Sprint(err) == "admin key already exists" {
			log.Println("admin already exists")
		} else {
			log.Fatal(err)
		}
	} else {
		log.Println("added admin key")
	}
	router := mux.NewRouter()

	router.PathPrefix("/files/").Methods("COPY").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err = handles.AuthenticateAndRoute("Copy", db, w, r)
			if err != nil {
				log.Println("failed to COPY: ", r.URL, ".", err)
				return
			}
		})

	router.PathPrefix("/files/").Methods("DELETE").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err = handles.AuthenticateAndRoute("Del", db, w, r)
			if err != nil {
				log.Println("failed to DELETE: ", r.URL, ".", err)
				return
			}
		})

	router.PathPrefix("/files/").Methods("GET").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("get hit")
			err = handles.AuthenticateAndRoute("Get", db, w, r)
			if err != nil {
				log.Println("failed to GET: ", r.URL, ".", err)
				return
			}
		})

	router.PathPrefix("/files/").Methods("HEAD").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err = handles.AuthenticateAndRoute("Head", db, w, r)
			if err != nil {
				log.Println("failed to HEAD: ", r.URL, ".", err)
				return
			}
		})

	router.PathPrefix("/files/").Methods("LOCK").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err = handles.AuthenticateAndRoute("Lock", db, w, r)
			if err != nil {
				log.Println("failed to LOCK: ", r.URL, ".", err)
				return
			}
		})

	router.PathPrefix("/files/").Methods("MKCOL").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err = handles.AuthenticateAndRoute("Mkcol", db, w, r)
			if err != nil {
				log.Println("failed to MKCOL: ", r.URL, ".", err)
				return
			}
		})

	router.PathPrefix("/files/").Methods("MOVE").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err = handles.AuthenticateAndRoute("Move", db, w, r)
			if err != nil {
				log.Println("failed to MOVE: ", r.URL, ".", err)
				return
			}
		})

	router.PathPrefix("/files/").Methods("OPTIONS").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err = handles.AuthenticateAndRoute("Options", db, w, r)
			if err != nil {
				log.Println("failed to OPTIONS: ", r.URL, ".", err)
				return
			}
		})

	router.PathPrefix("/files/").Methods("POST").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err = handles.AuthenticateAndRoute("Post", db, w, r)
			if err != nil {
				log.Println("failed to POST: ", r.URL, ".", err)
				return
			}
		})

	router.PathPrefix("/files/").Methods("PROPFIND").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err = handles.AuthenticateAndRoute("Propfind", db, w, r)
			if err != nil {
				log.Println("failed to PROPFIND: ", r.URL, ".", err)
				return
			}
		})

	router.PathPrefix("/files/").Methods("PUT").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err = handles.AuthenticateAndRoute("Put", db, w, r)
			if err != nil {
				log.Println("failed to PUT: ", r.URL, ".", err)
				return
			}
		})

	router.PathPrefix("/files/").Methods("TRACE").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err = handles.AuthenticateAndRoute("Trace", db, w, r)
			if err != nil {
				log.Println("failed to TRACE: ", r.URL, ".", err)
				return
			}
		})

	router.PathPrefix("/files/").Methods("UNLOCK").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err = handles.AuthenticateAndRoute("Unlock", db, w, r)
			if err != nil {
				log.Println("failed to UNLOCK: ", r.URL, ".", err)
				return
			}
		})

	router.HandleFunc("/addKey", func(w http.ResponseWriter, r *http.Request) {
		err = handles.AddKeyHandle(db, w, r)
		if err != nil {
			log.Println("failed to addKey:", r.URL, ".", err)
			return
		} else {
			log.Println("successful addKey:", r.URL)
		}
	})

	router.HandleFunc("/getKey", func(w http.ResponseWriter, r *http.Request) {
		err = handles.GetKeyHandle(db, w, r)
		if err != nil {
			log.Println("failed to getKey:", r.URL, ".", err)
			return
		} else {
			log.Println("successful getKey:", r.URL)
		}
	})

	router.HandleFunc("/deleteKey", func(w http.ResponseWriter, r *http.Request) {
		err = handles.DeleteKeyHandle(db, w, r)
		if err != nil {
			log.Println("failed to deleteKey:", r.URL, ".", err)
			return
		} else {
			log.Println("successful deleteKey:", r.URL)
		}
	})

	router.HandleFunc("/getChildKeys", func(w http.ResponseWriter, r *http.Request) {
		err = handles.GetChildrenHandle(db, w, r)
		if err != nil {
			log.Println("failed to getChildKeys:", r.URL, ".", err)
			return
		} else {
			log.Println("successful getChildKeys", r.URL)
		}
	})

	router.PathPrefix("/admin/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err = handles.AdminHandle(db, w, r)
		if err != nil {
			log.Println("failed to admin:", r.URL, ".", err)
			return
		} else {
			log.Println("successful admin", r.URL)
		}
	})

	handler := cors.Default().Handler(router)
	srv := &http.Server{
		Handler: handler,
		Addr:    "0.0.0.0:8080",
	}
	log.Println("proxy server up")
	log.Fatal(srv.ListenAndServe())
}
