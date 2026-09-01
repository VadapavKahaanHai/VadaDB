package main

import (
	"log"
	"net/http"
	"os"

	"vadadb/db"
	"vadadb/httpapi"
	"vadadb/web"
)

func main() {
	dir := os.Getenv("VADADB_DATA")
	if dir == "" {
		dir = "data"
	}
	database, err := db.Open(dir, 3)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	console, err := web.New(database)
	if err != nil {
		log.Fatal(err)
	}
	api := httpapi.New(database)
	mux := http.NewServeMux()
	mux.Handle("/kv/", api)
	mux.Handle("/scan", api)
	mux.Handle("/admin/", api)
	mux.Handle("/", console.Handler())
	address := os.Getenv("VADADB_ADDR")
	if address == "" {
		address = ":8080"
	}
	log.Printf("VadaDB listening on http://localhost%s (data: %s)", address, dir)
	log.Fatal(http.ListenAndServe(address, mux))
}
