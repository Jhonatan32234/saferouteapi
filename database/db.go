package database

import (
    "database/sql"
    "fmt"
    "log"
    "time"

    _ "github.com/lib/pq"
)

var DB *sql.DB

func Connect(databaseURL string) error {
    var err error
    
    DB, err = sql.Open("postgres", databaseURL)
    if err != nil {
        return fmt.Errorf("error abriendo conexión: %w", err)
    }

    // Configuración del pool
    DB.SetMaxOpenConns(25)
    DB.SetMaxIdleConns(5)
    DB.SetConnMaxLifetime(5 * time.Minute)

    // Verificar conexión
    if err = DB.Ping(); err != nil {
        return fmt.Errorf("error haciendo ping: %w", err)
    }

    log.Println("✅ Conexión a PostgreSQL establecida")
    return nil
}

func Close() {
    if DB != nil {
        DB.Close()
    }
}