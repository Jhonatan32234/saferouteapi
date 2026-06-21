package main

import (
	"encoding/json"
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
    // Cargar configuración
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

    // ==========================================
    // WebSocket - ANTES de los middlewares
    // ==========================================
    handlers.SetJWTSecret(cfg.JWTSecret) // Configurar el secret para WebSocket
    r.HandleFunc("/ws/alertas/{ruta_id}", handlers.WebSocketHandler())

    // ==========================================
    // Middlewares globales
    // ==========================================
    httpRouter := r.PathPrefix("/").Subrouter()
    httpRouter.Use(middleware.SecurityHeaders)
    httpRouter.Use(middleware.LoggingMiddleware)
    httpRouter.Use(middleware.RateLimitMiddleware(limiter))

    interno := httpRouter.PathPrefix("/api/internal").Subrouter()
    interno.Use(middleware.InternalAPIKeyMiddleware)

    interno.HandleFunc("/reportes", handlers.GetReportesHandler()).Methods("GET")
    interno.HandleFunc("/reportes/cercanos", handlers.GetReportesCercanosHandler()).Methods("GET")
    interno.HandleFunc("/reportes/estadisticas", handlers.GetEstadisticasHandler()).Methods("GET")
    interno.HandleFunc("/reportes/{id}", handlers.GetReporteHandler()).Methods("GET")

    // Documentación
    httpRouter.PathPrefix("/docs/").Handler(http.StripPrefix("/docs/", http.FileServer(http.Dir("static"))))
    httpRouter.HandleFunc("/api/docs", func(w http.ResponseWriter, r *http.Request) {
        http.ServeFile(w, r, "docs/api.md")
    })

    // === ENDPOINTS PÚBLICOS ===
    httpRouter.HandleFunc("/api/auth/login", handlers.LoginHandler(cfg.JWTSecret)).Methods("POST")
    httpRouter.HandleFunc("/api/auth/register", handlers.RegisterHandler()).Methods("POST")
    httpRouter.HandleFunc("/api/health", handlers.HealthHandler()).Methods("GET")

    // === ENDPOINTS PROTEGIDOS ===
    api := httpRouter.PathPrefix("/api").Subrouter()
    api.Use(middleware.AuthMiddleware(cfg.JWTSecret))

    // Perfil de usuario
    api.HandleFunc("/user/profile", handlers.GetUserProfileHandler()).Methods("GET")
    api.HandleFunc("/user/profile", handlers.UpdateUserProfileHandler()).Methods("PUT")

    // Historial de notificaciones
    api.HandleFunc("/user/notificaciones", handlers.GetHistorialNotificacionesHandler()).Methods("GET")
    api.HandleFunc("/user/notificaciones/marcar", handlers.MarcarNotificacionHandler()).Methods("PUT")
    api.HandleFunc("/user/notificaciones/marcar-todas", handlers.MarcarTodasNotificacionesHandler()).Methods("PUT")
    api.HandleFunc("/user/notificaciones/sincronizar", handlers.SincronizarNotificacionesHandler()).Methods("POST")

    // Suscripciones
    api.HandleFunc("/user/suscribir", handlers.SuscribirRutaHandler()).Methods("POST")
    api.HandleFunc("/user/desuscribir", handlers.DesuscribirRutaHandler()).Methods("DELETE")
    api.HandleFunc("/user/suscripciones", handlers.GetSuscripcionesHandler()).Methods("GET")

    api.HandleFunc("/user/zonas", handlers.ActualizarZonasUsuarioHandler()).Methods("POST")
    api.HandleFunc("/user/zonas", handlers.ObtenerZonasUsuarioHandler()).Methods("GET")

    // Predicciones (NUEVO)
api.HandleFunc("/predicciones/zonas", handlers.ProxyHandler(cfg.MotorPrediccionesURL+"/predicciones/zonas")).Methods("POST")
api.HandleFunc("/predicciones/perfil", handlers.ProxyHandler(cfg.MotorPrediccionesURL+"/predicciones/perfil")).Methods("POST")

    // Reportes
    api.HandleFunc("/reportes", handlers.CreateReporteHandler()).Methods("POST")
    api.HandleFunc("/reportes", handlers.GetReportesHandler()).Methods("GET")
    api.HandleFunc("/reportes/cercanos", handlers.GetReportesCercanosHandler()).Methods("GET")
    api.HandleFunc("/reportes/estadisticas", handlers.GetEstadisticasHandler()).Methods("GET")
    api.HandleFunc("/reportes/{id}", handlers.GetReporteHandler()).Methods("GET")
    api.HandleFunc("/reportes/{id}/validar", handlers.ValidarReporteHandler()).Methods("PUT")

    // Rutas
    api.HandleFunc("/rutas", handlers.GetRutasHandler(cfg.MotorRutasURL)).Methods("POST")

    // Admin
    api.HandleFunc("/admin/resumen", handlers.GetAdminResumenHandler(cfg.MotorNLPURL, cfg.MotorLLMURL)).Methods("GET")
    api.HandleFunc("/admin/buscar", handlers.BuscarReportesHandler(cfg.MotorNLPURL)).Methods("POST")

    // Debug
    r.HandleFunc("/api/debug/websocket", func(w http.ResponseWriter, r *http.Request) {
        estado := handlers.GetEstadoSuscriptores()
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(estado)
    }).Methods("GET")

    // CORS
    c := cors.New(cors.Options{
        AllowedOrigins:   []string{"*"},
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"*"},
        AllowCredentials: true,
    })

    handler := c.Handler(r)

    log.Printf("🚛 SafeRoute API Gateway v1.0.0")
    log.Printf("📡 Iniciando en puerto %s", cfg.Port)
    log.Printf("📚 Documentación: http://localhost:%s/docs/", cfg.Port)
    log.Fatal(http.ListenAndServe(":"+cfg.Port, handler))
}