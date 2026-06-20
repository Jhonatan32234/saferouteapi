package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	NombreKey   contextKey = "nombre"
	TipoKey     contextKey = "tipo"
)

// AuthMiddleware valida el token y guarda los claims en el contexto
func AuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                http.Error(w, "Authorization header required", http.StatusUnauthorized)
                return
            }

            parts := strings.Split(authHeader, " ")
            if len(parts) != 2 || parts[0] != "Bearer" {
                http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
                return
            }

            tokenString := parts[1]
            claims, err := ValidateToken(tokenString, jwtSecret)
            if err != nil {
                http.Error(w, "Invalid token", http.StatusUnauthorized)
                return
            }

            // Guardar en el contexto
            ctx := r.Context()
            
            // Extraer user_id
            userID, ok := claims["user_id"].(string)
            if !ok || userID == "" {
                http.Error(w, "User ID not found in token", http.StatusUnauthorized)
                return
            }
            ctx = context.WithValue(ctx, UserIDKey, userID)
            
            // Extraer nombre (opcional)
            if nombre, ok := claims["nombre"].(string); ok {
                ctx = context.WithValue(ctx, NombreKey, nombre)
            }
            
            // Extraer tipo (opcional)
            if tipo, ok := claims["tipo"].(string); ok {
                ctx = context.WithValue(ctx, TipoKey, tipo)
            }

            // Llamar al siguiente handler con el contexto actualizado
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// middleware/auth.go - GetUserID
func GetUserID(r *http.Request) string {
    if userID, ok := r.Context().Value(UserIDKey).(string); ok {
        return userID
    }
    // Si no está en el contexto, intentar obtener del header
    if userID := r.Header.Get("X-User-ID"); userID != "" {
        return userID
    }
    return ""
}

// GetNombre obtiene el nombre del contexto
func GetNombre(r *http.Request) string {
    if nombre, ok := r.Context().Value(NombreKey).(string); ok {
        return nombre
    }
    return ""
}

// GetTipo obtiene el tipo del contexto
func GetTipo(r *http.Request) string {
    if tipo, ok := r.Context().Value(TipoKey).(string); ok {
        return tipo
    }
    return ""
}

// ValidateToken valida un token JWT
func ValidateToken(tokenString string, jwtSecret string) (jwt.MapClaims, error) {
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return []byte(jwtSecret), nil
    })

    if err != nil {
        return nil, err
    }

    if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
        return claims, nil
    }

    return nil, fmt.Errorf("invalid token")
}

// GetUserIDFromToken extrae el user_id del token
func GetUserIDFromToken(tokenString string, jwtSecret string) (string, error) {
    claims, err := ValidateToken(tokenString, jwtSecret)
    if err != nil {
        return "", err
    }

    userID, ok := claims["user_id"].(string)
    if !ok {
        return "", fmt.Errorf("user_id not found in token")
    }

    return userID, nil
}