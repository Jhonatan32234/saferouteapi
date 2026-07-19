package middleware

import (
	"crypto/ed25519"
	"net/http"
	"strings"
)

func RoleMiddleware(pubKey ed25519.PublicKey, roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")

			claims, err := ValidateToken(tokenString, pubKey)
			if err != nil {
				http.Error(w, `{"error":"token inválido","code":401}`, http.StatusUnauthorized)
				return
			}

			tipo, ok := claims["tipo"].(string)
			if !ok {
				http.Error(w, `{"error":"sin permisos","code":403}`, http.StatusForbidden)
				return
			}

			for _, rol := range roles {
				if tipo == rol {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, `{"error":"acceso denegado: se requiere rol `+strings.Join(roles, " o ")+`","code":403}`, http.StatusForbidden)
		})
	}
}
