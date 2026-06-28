package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"saferoute/config"
	"saferoute/database"
	"saferoute/entities"
	"saferoute/handlers"
	"saferoute/middleware"
	"saferoute/repository"
	"saferoute/services"
)

//go:embed static
var staticFiles embed.FS

func main() {
	// ==========================================
	// 1. Cargar configuración
	// ==========================================
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Error cargando configuración:", err)
	}

	// ==========================================
	// 2. Decodificar clave de cifrado AES-256
	// ==========================================
	encryptionKey, err := entities.DecodeEncryptionKey(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("❌ Error con ENCRYPTION_KEY: %v. Asegúrate de que sea base64 de 32 bytes.", err)
	}
	log.Println("🔑 Clave de cifrado AES-256 cargada correctamente")

	// ==========================================
	// 3. Conectar a PostgreSQL
	// ==========================================
	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatal("Error conectando a base de datos:", err)
	}
	defer database.Close()

	// ==========================================
	// 4. Inicializar Repositorios (Capa de Datos)
	// ==========================================
	usuarioRepo := repository.NewUsuarioRepository(database.DB, encryptionKey)
	reporteRepo := repository.NewReporteRepository(database.DB)

	// ==========================================
	// 5. Inicializar Servicios (Lógica de Negocio)
	// ==========================================
	authSvc := services.NewAuthService(usuarioRepo, encryptionKey, cfg.JWTSecret)
	reporteSvc := services.NewReporteService(reporteRepo)
	userSvc := services.NewUserService(usuarioRepo, encryptionKey)

	// ==========================================
	// 6. Configurar Rate Limiter
	// ==========================================
	limiter := middleware.NewIPRateLimiter(5, 10)

	// ==========================================
	// 7. Configurar Router
	// ==========================================
	r := mux.NewRouter()

	// WebSocket - ANTES de los middlewares HTTP
	handlers.SetJWTSecret(cfg.JWTSecret)
	r.HandleFunc("/ws/alertas/{ruta_id}", handlers.WebSocketHandler())

	// ==========================================
	// 8. Middlewares globales
	// ==========================================
	httpRouter := r.PathPrefix("/").Subrouter()
	httpRouter.Use(middleware.SecurityHeaders)
	httpRouter.Use(middleware.LoggingMiddleware)
	httpRouter.Use(middleware.RateLimitMiddleware(limiter))

	// ==========================================
	// 9. ENDPOINTS INTERNOS (API Key)
	// ==========================================
	interno := httpRouter.PathPrefix("/api/internal").Subrouter()
	interno.Use(middleware.InternalAPIKeyMiddleware)

	interno.HandleFunc("/reportes", handlers.GetReportesHandler(reporteSvc)).Methods("GET")
	interno.HandleFunc("/reportes/cercanos", handlers.GetReportesCercanosHandler(reporteSvc)).Methods("GET")
	interno.HandleFunc("/reportes/estadisticas", handlers.GetEstadisticasHandler()).Methods("GET")
	interno.HandleFunc("/reportes/{id}", handlers.GetReporteHandler(reporteSvc)).Methods("GET")
	interno.HandleFunc("/usuarios", handlers.GetUsuariosInternoHandler()).Methods("GET")

	// ==========================================
	// DOCUMENTACIÓN SWAGGER (EMBEBIDA)
	// ==========================================
	docsFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Printf("⚠️ Error cargando documentación embebida: %v", err)
	} else {
		httpRouter.PathPrefix("/docs/").Handler(http.StripPrefix("/docs/", http.FileServer(http.FS(docsFS))))
	}

	// ==========================================
	// 10. ENDPOINTS PÚBLICOS
	// ==========================================
	httpRouter.HandleFunc("/api/auth/login", handlers.LoginHandler(authSvc, cfg.JWTSecret)).Methods("POST")
	httpRouter.HandleFunc("/api/auth/register", handlers.RegisterHandler(authSvc)).Methods("POST")
	httpRouter.HandleFunc("/api/health", handlers.HealthHandler()).Methods("GET", "HEAD")
	httpRouter.HandleFunc("/api/clusters", handlers.ProxyHandler(cfg.MotorRutasURL+"/clusters")).Methods("GET")

	// ==========================================
	// 11. ENDPOINTS PROTEGIDOS (JWT)
	// ==========================================
	api := httpRouter.PathPrefix("/api").Subrouter()
	api.Use(middleware.AuthMiddleware(cfg.JWTSecret))

	api.HandleFunc("/user/profile", handlers.GetUserProfileHandler(userSvc)).Methods("GET")
	api.HandleFunc("/user/profile", handlers.UpdateUserProfileHandler(userSvc)).Methods("PUT")
	api.HandleFunc("/user/notificaciones", handlers.GetHistorialNotificacionesHandler()).Methods("GET")
	api.HandleFunc("/user/notificaciones/marcar", handlers.MarcarNotificacionHandler()).Methods("PUT")
	api.HandleFunc("/user/notificaciones/marcar-todas", handlers.MarcarTodasNotificacionesHandler()).Methods("PUT")
	api.HandleFunc("/user/notificaciones/sincronizar", handlers.SincronizarNotificacionesHandler()).Methods("POST")
	api.HandleFunc("/user/suscribir", handlers.SuscribirRutaHandler()).Methods("POST")
	api.HandleFunc("/user/desuscribir", handlers.DesuscribirRutaHandler()).Methods("DELETE")
	api.HandleFunc("/user/suscripciones", handlers.GetSuscripcionesHandler()).Methods("GET")
	api.HandleFunc("/user/zonas", handlers.ActualizarZonasUsuarioHandler()).Methods("POST")
	api.HandleFunc("/user/zonas", handlers.ObtenerZonasUsuarioHandler()).Methods("GET")
	api.HandleFunc("/reportes", handlers.CreateReporteHandler(reporteSvc)).Methods("POST")
	api.HandleFunc("/reportes", handlers.GetReportesHandler(reporteSvc)).Methods("GET")
	api.HandleFunc("/reportes/cercanos", handlers.GetReportesCercanosHandler(reporteSvc)).Methods("GET")
	api.HandleFunc("/reportes/estadisticas", handlers.GetEstadisticasHandler()).Methods("GET")
	api.HandleFunc("/reportes/{id}", handlers.GetReporteHandler(reporteSvc)).Methods("GET")
	api.HandleFunc("/reportes/{id}/validar", handlers.ValidarReporteHandler(reporteSvc)).Methods("PUT")
	api.HandleFunc("/rutas", handlers.GetRutasHandler(cfg.MotorRutasURL)).Methods("POST")
	api.HandleFunc("/predicciones/zonas", handlers.ProxyHandler(cfg.MotorPrediccionesURL+"/predicciones/zonas")).Methods("POST")
	api.HandleFunc("/predicciones/perfil", handlers.ProxyHandler(cfg.MotorPrediccionesURL+"/predicciones/perfil")).Methods("POST")

	// ==========================================
	// 12. ENDPOINTS SOLO ADMIN (RBAC)
	// ==========================================
	apiAdmin := api.PathPrefix("/admin").Subrouter()
	apiAdmin.Use(middleware.RoleMiddleware(cfg.JWTSecret, "admin"))

	apiAdmin.HandleFunc("/resumen", handlers.GetAdminResumenHandler(cfg.MotorNLPURL, cfg.MotorLLMURL)).Methods("GET")
	apiAdmin.HandleFunc("/buscar", handlers.BuscarReportesHandler(cfg.MotorNLPURL)).Methods("POST")
	apiAdmin.HandleFunc("/registrar-conductor", handlers.RegistrarConductorHandler(authSvc)).Methods("POST")

	// ==========================================
	// 13. DEBUG
	// ==========================================
	r.HandleFunc("/api/debug/websocket", func(w http.ResponseWriter, r *http.Request) {
		estado := handlers.GetEstadoSuscriptores()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(estado)
	}).Methods("GET")

	// ==========================================
	// 14. CORS
	// ==========================================
	c := cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:8080",
			"http://localhost:3000",
			"http://127.0.0.1:8080",
			"http://127.0.0.1:3000",
			"http://localhost:5173",
			"https://saferoute-api-m4i5.onrender.com",
			"null",
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Internal-API-Key", "X-Requested-With"},
		AllowCredentials: true,
		MaxAge:           300,
	})

	handler := c.Handler(r)

	log.Printf("🚛 SafeRoute API Gateway v1.0.0")
	log.Printf("🏗️  Arquitectura: Repository → Service → Pipe → Handler")
	log.Printf("🔒 Seguridad: JWT + RBAC + AES-256 + CORS + Rate Limiting")
	log.Printf("📡 Iniciando en puerto %s", cfg.Port)
	log.Printf("📚 Documentación: http://localhost:%s/docs/docs.html", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, handler))
}