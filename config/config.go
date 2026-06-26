package config

import (
    "log"
    "os"
    "strconv"

    "github.com/joho/godotenv"
)

type Config struct {
    Port          string
    DatabaseURL   string
    JWTSecret     string
    MotorRutasURL string
    MotorNLPURL   string
    MotorLLMURL   string
    Environment   string
    RateLimit     RateLimitConfig
    InternalAPIKey string
    MotorPrediccionesURL  string
    EncryptionKey string // Clave AES-256 para cifrado de campos sensibles (base64, 32 bytes)

}

type RateLimitConfig struct {
    RequestsPerSecond int
    Burst             int
}

func Load() (*Config, error) {
    // Cargar .env solo en desarrollo
    if os.Getenv("ENVIRONMENT") != "production" {
        if err := godotenv.Load(); err != nil {
            log.Println("⚠️ No se pudo cargar .env, usando variables de entorno del sistema")
        }
    }

    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    databaseURL := os.Getenv("DATABASE_URL")
    if databaseURL == "" {
        log.Fatal("❌ DATABASE_URL es requerido")
    }

    jwtSecret := os.Getenv("JWT_SECRET")
    if jwtSecret == "" {
        log.Fatal("❌ JWT_SECRET es requerido")
    }

    motorRutasURL := os.Getenv("MOTOR_RUTAS_URL")
    if motorRutasURL == "" {
        motorRutasURL = "http://localhost:8000"
        log.Println("⚠️ MOTOR_RUTAS_URL no configurado, usando default:", motorRutasURL)
    }

    motorNLPURL := os.Getenv("MOTOR_NLP_URL")
    if motorNLPURL == "" {
        motorNLPURL = "http://localhost:8000"
        log.Println("⚠️ MOTOR_NLP_URL no configurado, usando default:", motorNLPURL)
    }

    motorLLMURL := os.Getenv("MOTOR_LLM_URL")
    if motorLLMURL == "" {
        motorLLMURL = "http://localhost:8000"
        log.Println("⚠️ MOTOR_LLM_URL no configurado, usando default:", motorLLMURL)
    }

    environment := os.Getenv("ENVIRONMENT")
    if environment == "" {
        environment = "development"
    }

    internal_api_key := os.Getenv("INTERNAL_API_KEY")
    if internal_api_key == "" {
        internal_api_key = "api_key_de_prueba"
    }

    motor_predicciones_url := os.Getenv("MOTOR_PREDICCIONES_URL")
    if motor_predicciones_url == ""{
        motor_predicciones_url = "localhost:8080"
    }
    // Configuración de rate limit
    requestsPerSecond, _ := strconv.Atoi(os.Getenv("RATE_LIMIT_REQUESTS"))
    if requestsPerSecond == 0 {
        requestsPerSecond = 10
    }

    burst, _ := strconv.Atoi(os.Getenv("RATE_LIMIT_BURST"))
    if burst == 0 {
        burst = 20
    }

    encryptionKey := os.Getenv("ENCRYPTION_KEY")
    if encryptionKey == "" {
        // Clave de desarrollo por defecto (32 bytes en base64). ¡Cambiar en producción!
        encryptionKey = "ZGV2ZWxvcG1lbnRLZXkxMjM0NTY3ODkwMTIzNA=="
        log.Println("⚠️ ENCRYPTION_KEY no configurado, usando clave de desarrollo. ¡No usar en producción!")
    }

    return &Config{
        Port:          port,
        DatabaseURL:   databaseURL,
        JWTSecret:     jwtSecret,
        MotorRutasURL: motorRutasURL,
        MotorNLPURL:   motorNLPURL,
        MotorLLMURL:   motorLLMURL,
        Environment:   environment,
        RateLimit: RateLimitConfig{
            RequestsPerSecond: requestsPerSecond,
            Burst:             burst,
        },
        InternalAPIKey:      internal_api_key,
        MotorPrediccionesURL: motor_predicciones_url,
        EncryptionKey:       encryptionKey,
    }, nil
}