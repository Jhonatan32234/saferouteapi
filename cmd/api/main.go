package main

import (
	"crypto/ed25519"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"golang.org/x/time/rate"

	"saferoute/internal/auth"
	"saferoute/internal/billing"
	"saferoute/internal/config"
	"saferoute/internal/database"
	"saferoute/internal/middleware"
	"saferoute/internal/motor"
	"saferoute/internal/reporte"
	"saferoute/internal/security"
	"saferoute/internal/user"
	"saferoute/internal/viaje"
)

//go:embed static
var staticFiles embed.FS

func main() {
	// ── Configuración ──────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Error cargando configuración:", err)
	}

	encryptionKey, err := security.DecodeEncryptionKey(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("Error con ENCRYPTION_KEY: %v. Asegúrate de que sea base64 de 32 bytes.", err)
	}
	log.Println("✅ Clave de cifrado AES-256 cargada correctamente")

	// Decodificar claves asimétricas Ed25519
	jwtPublicKeyBytes, err := base64.StdEncoding.DecodeString(cfg.JWTPublicKey)
	if err != nil {
		log.Fatalf("Error decodificando JWT_PUBLIC_KEY: %v. Debe ser base64 de 32 bytes.", err)
	}
	jwtPublicKey := ed25519.PublicKey(jwtPublicKeyBytes)

	var servicePrivateKey ed25519.PrivateKey
	if cfg.ServicePrivateKey != "" {
		servicePrivateKeyBytes, err := base64.StdEncoding.DecodeString(cfg.ServicePrivateKey)
		if err != nil {
			log.Fatalf("Error decodificando SERVICE_PRIVATE_KEY: %v. Debe ser base64 de 64 bytes.", err)
		}
		servicePrivateKey = ed25519.PrivateKey(servicePrivateKeyBytes)
	}

	log.Println("✅ Claves asimétricas Ed25519 cargadas correctamente")

	// ── Base de datos ──────────────────────────────────────────────────────────
	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatal("Error conectando a base de datos:", err)
	}
	defer database.Close()
	db := database.DB

	// ── Repositorios ───────────────────────────────────────────────────────────
	userRepo := user.NewRepository(db, encryptionKey)
	reporteRepo := reporte.NewRepository(db)
	viajeRepo := viaje.NewRepository(db)

	// ── Servicios ──────────────────────────────────────────────────────────────
	motorSvc := motor.NewService(cfg.MotorNLPURL, cfg.MotorPrediccionesURL, cfg.InternalAPIKey)

	// Motor adapters (implementan interfaces locales con el motorSvc genérico)
	reporteMotorSvc := &reporteMotorAdapter{svc: motorSvc}

	userSvc := user.NewService(userRepo)
	authSvc := auth.NewAuthService(cfg.AuthServiceURL, cfg.InternalAPIKey, servicePrivateKey)
	reporteSvc := reporte.NewService(reporteRepo, userRepo, reporteMotorSvc)
	viajeSvc := viaje.NewService(viajeRepo, userRepo)

	// ── Monitor de heartbeat ───────────────────────────────────────────────────
	viaje.StartSignalTimeoutMonitor(db, 1*time.Minute, 5*time.Minute)

	// ── WebSocket manager ──────────────────────────────────────────────────────
	wsAdapter := &wsMotorAdapter{svc: motorSvc}
	wsMgr := viaje.NewWebSocketManager(db, viajeSvc, wsAdapter, jwtPublicKey)

	// ── Billing (Facturación / Planes Empresariales) ──────────────────────────
	billingRepo := billing.NewRepository(db)
	billingStripeCfg := &billing.StripeConfig{
		SecretKey:      cfg.StripeSecretKey,
		WebhookSecret:  cfg.StripeWebhookSecret,
		PriceBasico:    cfg.StripePriceBasico,
		PricePro:       cfg.StripePricePro,
		PriceExtra:     cfg.StripePriceExtra,
		SuccessURL:     cfg.StripeSuccessURL,
		CancelURL:      cfg.StripeCancelURL,
	}
	billingSvc := billing.NewService(billingRepo, billingStripeCfg)
	billingMiddleware := middleware.RequireActiveSubscription(billingSvc)
	billingHandler := billing.NewHandler(billingSvc)

	// ── Handlers ───────────────────────────────────────────────────────────────
	authHandler := auth.NewHandler(authSvc, cfg.JWTSecret)
	userHandler := user.NewHandler(userSvc)
	reporteHandler := reporte.NewHandler(reporteSvc, &wsNotifierAdapter{mgr: wsMgr})
	adminReporteHandler := reporte.NewAdminHandler(cfg.MotorNLPURL, cfg.MotorLLMURL)
	viajeHandler := viaje.NewHandler(viajeSvc)
	motorHandler := motor.NewHandler(cfg.MotorRutasURL)

	// ── Rate limiter ───────────────────────────────────────────────────────────
	limiter := middleware.NewIPRateLimiter(
		rate.Limit(cfg.RateLimit.RequestsPerSecond),
		cfg.RateLimit.Burst,
	)

	// ── Router ─────────────────────────────────────────────────────────────────
	r := mux.NewRouter()

	// WebSocket (sin auth middleware, maneja su propio token)
	r.HandleFunc("/ws/alertas/{ruta_id}", wsMgr.WebSocketHandler())

	// Sub-router principal con middlewares
	httpRouter := r.PathPrefix("/").Subrouter()
	httpRouter.Use(middleware.SecurityHeaders)
	httpRouter.Use(middleware.LoggingMiddleware)
	httpRouter.Use(middleware.RateLimitMiddleware(limiter))

	// ── Rutas internas (API Key) ────────────────────────────────────────────────
	interno := httpRouter.PathPrefix("/api/internal").Subrouter()
	interno.Use(middleware.InternalAPIKeyMiddleware)
	interno.HandleFunc("/reportes", reporteHandler.GetReportesHandler()).Methods("GET")
	interno.HandleFunc("/reportes/cercanos", reporteHandler.GetReportesCercanosHandler()).Methods("GET")
	interno.HandleFunc("/reportes/estadisticas", reporteHandler.GetEstadisticasHandler()).Methods("GET")
	interno.HandleFunc("/reportes/{id}", reporteHandler.GetReporteHandler()).Methods("GET")
	interno.HandleFunc("/usuarios", userHandler.GetUsuariosInternoHandler()).Methods("GET")

	// ── Documentación estática ─────────────────────────────────────────────────
	docsFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Printf("Error cargando documentación embebida: %v", err)
	} else {
		httpRouter.PathPrefix("/docs/").Handler(http.StripPrefix("/docs/", http.FileServer(http.FS(docsFS))))
	}

	// ── Rutas públicas ─────────────────────────────────────────────────────────
	httpRouter.HandleFunc("/api/auth/login", authHandler.LoginHandler()).Methods("POST")
	httpRouter.HandleFunc("/api/auth/register", authHandler.RegisterHandler()).Methods("POST")
	httpRouter.HandleFunc("/api/health", healthHandler(db)).Methods("GET", "HEAD")
	httpRouter.HandleFunc("/api/clusters", motor.ProxyHandler(cfg.MotorRutasURL+"/clusters")).Methods("GET")
	httpRouter.HandleFunc("/api/auth/register-admin", authHandler.RegistrarAdminPublicoHandler()).Methods("POST") 

	// ── Rutas públicas de facturación ──────────────────────────────────────────
	httpRouter.HandleFunc("/api/billing/plans", billingHandler.GetPlanesHandler()).Methods("GET")
	httpRouter.HandleFunc("/api/billing/metodos-pago", billingHandler.GetMetodosPagoHandler()).Methods("GET")
	httpRouter.HandleFunc("/api/billing/precios/calcular", billingHandler.CalcularPrecioHandler()).Methods("GET")
	httpRouter.HandleFunc("/api/webhooks/stripe", billingHandler.WebhookStripeHandler()).Methods("POST")

	// ── Rutas autenticadas ─────────────────────────────────────────────────────
	// ── Rutas autenticadas (conductores + admins) ────────────────────────────
api := httpRouter.PathPrefix("/api").Subrouter()
api.Use(middleware.AuthMiddleware(jwtPublicKey))

// ✅ Rutas para TODOS los usuarios autenticados (sin billing)
// Usuario
api.HandleFunc("/user/profile", userHandler.GetUserProfileHandler()).Methods("GET")
api.HandleFunc("/user/profile", userHandler.UpdateUserProfileHandler()).Methods("PUT")
api.HandleFunc("/user/notificaciones", userHandler.GetHistorialNotificacionesHandler()).Methods("GET")
api.HandleFunc("/user/notificaciones/marcar", userHandler.MarcarNotificacionHandler()).Methods("PUT")
api.HandleFunc("/user/notificaciones/marcar-todas", userHandler.MarcarTodasNotificacionesHandler()).Methods("PUT")
api.HandleFunc("/user/notificaciones/sincronizar", userHandler.SincronizarNotificacionesHandler()).Methods("POST")
api.HandleFunc("/user/suscribir", userHandler.SuscribirRutaHandler()).Methods("POST")
api.HandleFunc("/user/desuscribir", userHandler.DesuscribirRutaHandler()).Methods("DELETE")
api.HandleFunc("/user/suscripciones", userHandler.GetSuscripcionesHandler()).Methods("GET")
api.HandleFunc("/user/zonas", userHandler.ActualizarZonasUsuarioHandler()).Methods("POST")
api.HandleFunc("/user/zonas", userHandler.ObtenerZonasUsuarioHandler()).Methods("GET")
api.HandleFunc("/user/destinos", userHandler.GuardarDestinoRecenteHandler()).Methods("POST")
api.HandleFunc("/user/destinos", userHandler.GetDestinosRecientesHandler()).Methods("GET")
api.HandleFunc("/user/destinos", userHandler.EliminarDestinoRecenteHandler()).Methods("DELETE")

// Rutas y predicciones
api.HandleFunc("/rutas", motorHandler.GetRutasHandler()).Methods("POST")
api.HandleFunc("/predicciones/zonas", motor.ProxyHandler(cfg.MotorPrediccionesURL+"/predicciones/zonas")).Methods("POST")
api.HandleFunc("/predicciones/perfil", motor.ProxyHandler(cfg.MotorPrediccionesURL+"/predicciones/perfil")).Methods("POST")

// Viajes
api.HandleFunc("/viajes/iniciar", viajeHandler.IniciarViajeHandler()).Methods("POST")
api.HandleFunc("/viajes/finalizar", viajeHandler.FinalizarViajeHandler()).Methods("POST")
api.HandleFunc("/viajes/activo", viajeHandler.GetActiveViajeHandler()).Methods("GET")

// Reportes
api.HandleFunc("/reportes", reporteHandler.CreateReporteHandler()).Methods("POST")
api.HandleFunc("/reportes", reporteHandler.GetReportesHandler()).Methods("GET")
api.HandleFunc("/reportes/cercanos", reporteHandler.GetReportesCercanosHandler()).Methods("GET")
api.HandleFunc("/reportes/estadisticas", reporteHandler.GetEstadisticasHandler()).Methods("GET")
api.HandleFunc("/reportes/{id}", reporteHandler.GetReporteHandler()).Methods("GET")
api.HandleFunc("/reportes/{id}/validar", reporteHandler.ValidarReporteHandler()).Methods("PUT")

// ── Subrouter para rutas que requieren suscripción activa ────────────────
apiBilling := api.PathPrefix("").Subrouter()
apiBilling.Use(billingMiddleware)

// Facturación
apiBilling.HandleFunc("/billing/empresa", billingHandler.GetMiEmpresaHandler()).Methods("GET")
apiBilling.HandleFunc("/billing/empresa/crear", billingHandler.CrearSuscripcionHandler()).Methods("POST")
apiBilling.HandleFunc("/billing/empresa/cambiar-plan", billingHandler.CambiarPlanHandler()).Methods("PUT")
apiBilling.HandleFunc("/billing/empresa/conductores", billingHandler.AgregarConductoresHandler()).Methods("POST")
apiBilling.HandleFunc("/billing/empresa/cancelar", billingHandler.CancelarSuscripcionHandler()).Methods("POST")
apiBilling.HandleFunc("/billing/facturas", billingHandler.GetFacturasHandler()).Methods("GET")
apiBilling.HandleFunc("/billing/historial", billingHandler.GetHistorialHandler()).Methods("GET")

// ── Rutas de administrador ─────────────────────────────────────────────────
apiAdmin := api.PathPrefix("/admin").Subrouter()
apiAdmin.Use(middleware.RoleMiddleware(jwtPublicKey, "admin"))
apiAdmin.Use(billingMiddleware)  // Los admins también necesitan suscripción
apiAdmin.HandleFunc("/resumen", adminReporteHandler.GetAdminResumenHandler()).Methods("GET")
apiAdmin.HandleFunc("/buscar", adminReporteHandler.BuscarReportesHandler()).Methods("POST")
apiAdmin.HandleFunc("/registrar-conductor", authHandler.RegistrarConductorHandler(billingSvc)).Methods("POST")
apiAdmin.HandleFunc("/viajes/activos", viajeHandler.GetActiveViajesAdminHandler(billingSvc)).Methods("GET")
apiAdmin.HandleFunc("/notificar-conductor", wsMgr.NotificarConductorHandler()).Methods("POST")
apiAdmin.HandleFunc("/billing/empresas", billingHandler.AdminListEmpresasHandler()).Methods("GET")
apiAdmin.HandleFunc("/conductores", userHandler.GetConductoresEmpresaHandler(billingSvc)).Methods("GET")
apiAdmin.HandleFunc("/conductores/{id}/perfil", motorHandler.GetPerfilConductorHandler()).Methods("GET")
	// ── Debug ──────────────────────────────────────────────────────────────────
	r.HandleFunc("/api/debug/websocket", func(w http.ResponseWriter, r *http.Request) {
		estado := wsMgr.GetEstadoSuscriptores()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(estado)
	}).Methods("GET")

	// ── CORS ───────────────────────────────────────────────────────────────────
	c := cors.New(cors.Options{
		AllowOriginFunc: func(origin string) bool {
			return true
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Internal-API-Key", "X-Requested-With"},
		AllowCredentials: false,
		MaxAge:           300,
	})

	handler := c.Handler(r)

	log.Printf("🚀 SafeRoute API v2.0 — Arquitectura Limpia")
	log.Printf("📦 Módulos: auth · user · reporte · viaje · motor")
	log.Printf("🔒 Seguridad: JWT + RBAC + AES-256 + CORS + Rate Limiting")
	log.Printf("🌐 Iniciando en puerto %s", cfg.Port)
	log.Printf("📚 Documentación: http://localhost:%s/docs/docs.html", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, handler))
}

// ── Adapters (bridge entre interfaces de módulos y el servicio de motor) ───────

// reporteMotorAdapter adapta motor.Service a reporte.MotorService
type reporteMotorAdapter struct {
	svc motor.Service
}

func (a *reporteMotorAdapter) SyncReporteCreado(rep reporte.ReporteResponse) {
	a.svc.SyncReporteCreado(rep)
}

func (a *reporteMotorAdapter) SyncReporteValidado(reporteID string, vigente bool) {
	a.svc.SyncReporteValidado(reporteID, vigente)
}

// wsMotorAdapter adapta motor.Service a viaje.MotorService
type wsMotorAdapter struct {
	svc motor.Service
}

func (a *wsMotorAdapter) SyncInteraccion(tipo, userID, rutaID string, data map[string]interface{}) {
	a.svc.SyncInteraccion(tipo, userID, rutaID, data)
}

func (a *wsMotorAdapter) SyncReporteValidado(reporteID string, vigente bool) {
	a.svc.SyncReporteValidado(reporteID, vigente)
}

func (a *wsMotorAdapter) SyncReporteCreado(rep reporte.ReporteResponse) {
	a.svc.SyncReporteCreado(rep)
}

// wsNotifierAdapter adapta viaje.WebSocketManager a reporte.WSNotifier
type wsNotifierAdapter struct {
	mgr *viaje.WebSocketManager
}

func (a *wsNotifierAdapter) BroadcastNotificacion(n reporte.NotificacionAlerta) {
	a.mgr.BroadcastNotificacion(n)
}

func (a *wsNotifierAdapter) NotifyRutasCercanas(rep reporte.ReporteResponse) {
	a.mgr.NotifyRutasCercanas(rep)
}

// healthHandler returns a JSON health check
func healthHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "ok"
		dbStatus := "ok"
		if db == nil || db.Ping() != nil {
			dbStatus = "error"
			status = "degraded"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":   status,
			"database": dbStatus,
			"version":  "2.0.0",
		})
	}
}
