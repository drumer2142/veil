package main

import (
	"embed"
	"flag"
	"log"
	"net/http"
	"path/filepath"
)

//go:embed web/*
var webFS embed.FS

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dataDir := flag.String("data", "./data", "directory for SQLite database and icons")
	flag.Parse()

	dataPath, err := filepath.Abs(*dataDir)
	if err != nil {
		log.Fatal(err)
	}

	db, err := openDB(dataPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	st := newStore(db, dataPath)
	srv := newServer(st)

	log.Printf("dashboard listening on %s (data: %s)", *addr, dataPath)
	log.Fatal(http.ListenAndServe(*addr, srv))
}
