package middleware

import (
	"context"
	"crypto/ed25519"
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

func AuthMiddleware(pubKey ed25519.PublicKey) func(http.Handler) http.Handler {
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
            claims, err := ValidateToken(tokenString, pubKey)
            if err != nil {
                http.Error(w, "Invalid token", http.StatusUnauthorized)
                return
            }

            ctx := r.Context()
            
            userID, ok := claims["user_id"].(string)
            if !ok || userID == "" {
                http.Error(w, "User ID not found in token", http.StatusUnauthorized)
                return
            }
            ctx = context.WithValue(ctx, UserIDKey, userID)
            
            if nombre, ok := claims["nombre"].(string); ok {
                ctx = context.WithValue(ctx, NombreKey, nombre)
            }
            
            if tipo, ok := claims["tipo"].(string); ok {
                ctx = context.WithValue(ctx, TipoKey, tipo)
            }

            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func GetUserID(r *http.Request) string {
    if userID, ok := r.Context().Value(UserIDKey).(string); ok {
        return userID
    }
    if userID := r.Header.Get("X-User-ID"); userID != "" {
        return userID
    }
    return ""
}

func GetNombre(r *http.Request) string {
    if nombre, ok := r.Context().Value(NombreKey).(string); ok {
        return nombre
    }
    return ""
}

func GetTipo(r *http.Request) string {
    if tipo, ok := r.Context().Value(TipoKey).(string); ok {
        return tipo
    }
    return ""
}

func ValidateToken(tokenString string, pubKey ed25519.PublicKey) (jwt.MapClaims, error) {
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return pubKey, nil
    })

    if err != nil {
        return nil, err
    }

    if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
        return claims, nil
    }

    return nil, fmt.Errorf("invalid token")
}

func GetUserIDFromToken(tokenString string, pubKey ed25519.PublicKey) (string, error) {
    claims, err := ValidateToken(tokenString, pubKey)
    if err != nil {
        return "", err
    }

    userID, ok := claims["user_id"].(string)
    if !ok {
        return "", fmt.Errorf("user_id not found in token")
    }

    return userID, nil
}
