package config

import (
    "os"
    "github.com/joho/godotenv"
)

type Config struct {
    Port           string
    DatabaseURL    string
    JWTSecret      string
    MotorRutasURL  string
    MotorNLPURL    string
    MotorLLMURL    string
    Environment    string
}

func Load() (*Config, error) {
    // Cargar .env solo en desarrollo
    godotenv.Load()

    cfg := &Config{
        Port:           getEnv("PORT", "8080"),
        DatabaseURL:    getEnv("DATABASE_URL", ""),
        JWTSecret:      getEnv("JWT_SECRET", ""),
        MotorRutasURL:  getEnv("MOTOR_RUTAS_URL", "http://localhost:8000"),
        MotorNLPURL:    getEnv("MOTOR_NLP_URL", "http://localhost:8001"),
        MotorLLMURL:    getEnv("MOTOR_LLM_URL", "http://localhost:8002"),
        Environment:    getEnv("ENVIRONMENT", "development"),
    }

    // Validaciones críticas
    if cfg.DatabaseURL == "" {
        panic("DATABASE_URL es requerida")
    }
    if cfg.JWTSecret == "" {
        panic("JWT_SECRET es requerido")
    }

    return cfg, nil
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}