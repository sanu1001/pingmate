package config

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func ConnectDB() {
	db, err := sql.Open("postgres", App.DatabaseURL)
	if err != nil {
		log.Fatalf("DB open error: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("DB ping failed — is postgres running? %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)

	DB = db
	log.Println("✓ PostgreSQL connected")
}
