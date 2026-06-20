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
        motorRutasURL = "https://saferoute-motor-rutas.onrender.com"
        log.Println("⚠️ MOTOR_RUTAS_URL no configurado, usando default:", motorRutasURL)
    }

    motorNLPURL := os.Getenv("MOTOR_NLP_URL")
    if motorNLPURL == "" {
        motorNLPURL = "https://saferoute-motor-nlp.onrender.com"
        log.Println("⚠️ MOTOR_NLP_URL no configurado, usando default:", motorNLPURL)
    }

    motorLLMURL := os.Getenv("MOTOR_LLM_URL")
    if motorLLMURL == "" {
        motorLLMURL = "https://saferoute-motor-llm.onrender.com"
        log.Println("⚠️ MOTOR_LLM_URL no configurado, usando default:", motorLLMURL)
    }

    environment := os.Getenv("ENVIRONMENT")
    if environment == "" {
        environment = "development"
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
    }, nil
}