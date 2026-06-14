package middleware

import (
    "context"
    "net/http"
    "strings"

    "github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func AuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                http.Error(w, `{"error":"token requerido","code":401}`, http.StatusUnauthorized)
                return
            }

            tokenString := strings.TrimPrefix(authHeader, "Bearer ")
            if tokenString == authHeader {
                http.Error(w, `{"error":"formato de token inválido","code":401}`, http.StatusUnauthorized)
                return
            }

            token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
                if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                    return nil, jwt.ErrSignatureInvalid
                }
                return []byte(jwtSecret), nil
            })

            if err != nil || !token.Valid {
                http.Error(w, `{"error":"token inválido o expirado","code":401}`, http.StatusUnauthorized)
                return
            }

            claims, ok := token.Claims.(jwt.MapClaims)
            if !ok {
                http.Error(w, `{"error":"claims inválidos","code":401}`, http.StatusUnauthorized)
                return
            }

            userID, ok := claims["user_id"].(string)
            if !ok {
                http.Error(w, `{"error":"user_id no encontrado en token","code":401}`, http.StatusUnauthorized)
                return
            }

            ctx := context.WithValue(r.Context(), UserIDKey, userID)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func GetUserID(r *http.Request) string {
    if userID, ok := r.Context().Value(UserIDKey).(string); ok {
        return userID
    }
    return ""
}