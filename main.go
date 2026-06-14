package main

import (
    "log"
    "net/http"

    "github.com/gorilla/mux"
    "github.com/rs/cors"

    "saferoute/config"
    "saferoute/database"
    "saferoute/handlers"
    "saferoute/middleware"
)

func main() {
    // Cargar configuración desde variables de entorno
    cfg, err := config.Load()
    if err != nil {
        log.Fatal("Error cargando configuración:", err)
    }

    // Conectar a PostgreSQL
    if err := database.Connect(cfg.DatabaseURL); err != nil {
        log.Fatal("Error conectando a base de datos:", err)
    }
    defer database.Close()

    // Configurar rate limiter
    limiter := middleware.NewIPRateLimiter(5, 10)

    // Router
    r := mux.NewRouter()

    // Middleware global
    r.Use(middleware.SecurityHeaders)
    r.Use(middleware.LoggingMiddleware)
    r.Use(middleware.RateLimitMiddleware(limiter))

    // Documentación
    r.PathPrefix("/docs/").Handler(http.StripPrefix("/docs/", http.FileServer(http.Dir("static"))))
    r.HandleFunc("/api/docs", func(w http.ResponseWriter, r *http.Request) {
        http.ServeFile(w, r, "docs/api.md")
    })

    // === ENDPOINTS PÚBLICOS ===
    r.HandleFunc("/api/auth/login", handlers.LoginHandler(cfg.JWTSecret)).Methods("POST")
    r.HandleFunc("/api/auth/register", handlers.RegisterHandler()).Methods("POST")
    r.HandleFunc("/api/health", handlers.HealthHandler()).Methods("GET")

    // === ENDPOINTS PROTEGIDOS ===
    api := r.PathPrefix("/api").Subrouter()
    api.Use(middleware.AuthMiddleware(cfg.JWTSecret))

    // Reportes
    api.HandleFunc("/reportes", handlers.CreateReporteHandler()).Methods("POST")
    api.HandleFunc("/reportes", handlers.GetReportesHandler()).Methods("GET")
    api.HandleFunc("/reportes/{id}/validar", handlers.ValidarReporteHandler()).Methods("PUT")

    // Rutas
    api.HandleFunc("/rutas", handlers.GetRutasHandler(cfg.MotorRutasURL)).Methods("POST")

    // Admin
    api.HandleFunc("/admin/resumen", handlers.GetAdminResumenHandler(cfg.MotorLLMURL)).Methods("GET")

    // WebSocket (no protegido por JWT tradicional, se valida al conectar)
    r.HandleFunc("/ws/alertas/{ruta_id}", handlers.WebSocketHandler())

    // CORS
    c := cors.New(cors.Options{
        AllowedOrigins:   []string{"*"},
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Authorization", "Content-Type"},
        AllowCredentials: true,
    })

    handler := c.Handler(r)

    log.Printf("🚛 SafeRoute API Gateway v1.0.0")
    log.Printf("📡 Iniciando en puerto %s", cfg.Port)
    log.Printf("📚 Documentación: http://localhost:%s/docs/", cfg.Port)
    log.Fatal(http.ListenAndServe(":"+cfg.Port, handler))
}