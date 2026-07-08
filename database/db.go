package database

import (
    "database/sql"
    "fmt"
    "log"
    "sync"
    "time"

    _ "github.com/lib/pq"
)

var (
    DB        *sql.DB
    dbURL     string
    dbMutex   sync.RWMutex
    isConnected bool
)

func Connect(databaseURL string) error {
    dbMutex.Lock()
    defer dbMutex.Unlock()
    
    dbURL = databaseURL
    
    var err error
    DB, err = sql.Open("postgres", databaseURL)
    if err != nil {
        return fmt.Errorf("error abriendo conexión: %w", err)
    }

    DB.SetMaxOpenConns(25)
    DB.SetMaxIdleConns(5)
    DB.SetConnMaxLifetime(5 * time.Minute)
    DB.SetConnMaxIdleTime(2 * time.Minute)

    if err = DB.Ping(); err != nil {
        return fmt.Errorf("error haciendo ping: %w", err)
    }

    isConnected = true
    log.Println("Conexión a PostgreSQL establecida")

    return nil
}

func Close() {
    dbMutex.Lock()
    defer dbMutex.Unlock()
    
    if DB != nil {
        DB.Close()
        DB = nil
        isConnected = false
    }
}

func GetDB() *sql.DB {
    dbMutex.RLock()
    defer dbMutex.RUnlock()
    
    if DB == nil {
        log.Println("⚠️ Base de datos no conectada")
        return nil
    }
    
    if err := DB.Ping(); err != nil {
        log.Printf("⚠️ Ping falló: %v", err)
        return nil
    }
    
    return DB
}

func EnsureConnection() error {
    dbMutex.Lock()
    defer dbMutex.Unlock()
    
    if DB == nil {
        if dbURL == "" {
            return fmt.Errorf("no hay URL de conexión configurada")
        }
        log.Println("Reconectando a PostgreSQL...")
        
        var err error
        DB, err = sql.Open("postgres", dbURL)
        if err != nil {
            return fmt.Errorf("error abriendo conexión: %w", err)
        }
        
        DB.SetMaxOpenConns(25)
        DB.SetMaxIdleConns(5)
        DB.SetConnMaxLifetime(5 * time.Minute)
        DB.SetConnMaxIdleTime(2 * time.Minute)
        
        if err = DB.Ping(); err != nil {
            return fmt.Errorf("error haciendo ping: %w", err)
        }
        
        isConnected = true
        log.Println("Reconexión a PostgreSQL exitosa")
        return nil
    }
    
    if err := DB.Ping(); err != nil {
        log.Printf("Ping falló: %v, reconectando...", err)
        DB.Close()
        DB = nil
        
        if dbURL == "" {
            return fmt.Errorf("no hay URL de conexión configurada")
        }
        
        var err2 error
        DB, err2 = sql.Open("postgres", dbURL)
        if err2 != nil {
            return fmt.Errorf("error abriendo conexión: %w", err2)
        }
        
        DB.SetMaxOpenConns(25)
        DB.SetMaxIdleConns(5)
        DB.SetConnMaxLifetime(5 * time.Minute)
        DB.SetConnMaxIdleTime(2 * time.Minute)
        
        if err2 = DB.Ping(); err2 != nil {
            return fmt.Errorf("error haciendo ping: %w", err2)
        }
        
        isConnected = true
        log.Println("Reconexión a PostgreSQL exitosa")
        return nil
    }
    
    return nil
}

func IsConnected() bool {
    dbMutex.RLock()
    defer dbMutex.RUnlock()
    return isConnected && DB != nil
}