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

    // Configuración del pool
    DB.SetMaxOpenConns(25)
    DB.SetMaxIdleConns(5)
    DB.SetConnMaxLifetime(5 * time.Minute)
    DB.SetConnMaxIdleTime(2 * time.Minute)

    // Verificar conexión
    if err = DB.Ping(); err != nil {
        return fmt.Errorf("error haciendo ping: %w", err)
    }

    isConnected = true
    log.Println("✅ Conexión a PostgreSQL establecida")

    // Ejecutar migraciones automáticamente
    if err := RunMigrations(DB); err != nil {
        log.Printf("⚠️ Error al ejecutar migraciones de inicio: %v", err)
    }

    return nil
}

// RunMigrations ejecuta las sentencias SQL necesarias para estructurar las nuevas tablas de viajes y soporte PostGIS
func RunMigrations(db *sql.DB) error {
	migrations := []string{
		`CREATE EXTENSION IF NOT EXISTS postgis;`,
		`CREATE TABLE IF NOT EXISTS viajes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL,
			ruta_id VARCHAR(255) NOT NULL,
			origen_lat DOUBLE PRECISION NOT NULL,
			origen_lon DOUBLE PRECISION NOT NULL,
			destino_lat DOUBLE PRECISION NOT NULL,
			destino_lon DOUBLE PRECISION NOT NULL,
			polyline_ruta TEXT NOT NULL,
			geom_ruta GEOMETRY(LineString, 4326),
			estado VARCHAR(50) DEFAULT 'activo',
			fecha_inicio TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			fecha_fin TIMESTAMP WITH TIME ZONE,
			ultimo_heartbeat TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT chk_estado CHECK (estado IN ('activo', 'finalizado', 'desviado', 'parada_tecnica', 'contacto_perdido', 'cancelado'))
		);`,
		`CREATE TABLE IF NOT EXISTS historial_viaje_coordenadas (
			id BIGSERIAL PRIMARY KEY,
			viaje_id UUID NOT NULL REFERENCES viajes(id) ON DELETE CASCADE,
			latitud DOUBLE PRECISION NOT NULL,
			longitud DOUBLE PRECISION NOT NULL,
			velocidad_kmh DOUBLE PRECISION,
			timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_viajes_estado ON viajes(estado);`,
		`CREATE INDEX IF NOT EXISTS idx_viajes_user_id ON viajes(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_historial_viaje_id ON historial_viaje_coordenadas(viaje_id);`,
		`CREATE INDEX IF NOT EXISTS idx_viajes_geom ON viajes USING gist(geom_ruta);`,
	}

	for i, q := range migrations {
		if _, err := db.Exec(q); err != nil {
			// Si falla PostGIS (extensión), es probable que el usuario de la BD no sea superusuario.
			// Loggear advertencia y continuar, ya que podría estar habilitado de antemano o en Neon.
			if i == 0 {
				log.Printf("⚠️ Advertencia: No se pudo verificar la extensión PostGIS: %v. Continuando...", err)
				continue
			}
			return fmt.Errorf("error ejecutando migración %d: %w", i, err)
		}
	}
	log.Println("✅ Migraciones de viajes e historial ejecutadas/verificadas correctamente")
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

// GetDB retorna la conexión a la base de datos con reconexión automática
func GetDB() *sql.DB {
    dbMutex.RLock()
    defer dbMutex.RUnlock()
    
    if DB == nil {
        log.Println("⚠️ Base de datos no conectada")
        return nil
    }
    
    // Verificar que la conexión esté viva
    if err := DB.Ping(); err != nil {
        log.Printf("⚠️ Ping falló: %v", err)
        // No reconectar aquí porque tendríamos que tener el mutex de escritura
        // y podría causar deadlock
        return nil
    }
    
    return DB
}

// EnsureConnection asegura que la conexión esté activa, si no, la reconecta
func EnsureConnection() error {
    dbMutex.Lock()
    defer dbMutex.Unlock()
    
    // Si la conexión es nula o está cerrada, reconectar
    if DB == nil {
        if dbURL == "" {
            return fmt.Errorf("no hay URL de conexión configurada")
        }
        log.Println("🔄 Reconectando a PostgreSQL...")
        
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
        log.Println("✅ Reconexión a PostgreSQL exitosa")
        return nil
    }
    
    // Verificar que la conexión esté viva
    if err := DB.Ping(); err != nil {
        log.Printf("⚠️ Ping falló: %v, reconectando...", err)
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
        log.Println("✅ Reconexión a PostgreSQL exitosa")
        return nil
    }
    
    return nil
}

// IsConnected retorna el estado de la conexión
func IsConnected() bool {
    dbMutex.RLock()
    defer dbMutex.RUnlock()
    return isConnected && DB != nil
}