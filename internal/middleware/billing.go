// saferoute/internal/middleware/billing.go

package middleware

import (
    "net/http"
    "strings"
)

// RequireActiveSubscription verifica que el admin tenga un plan activo.
// Los conductores NO son verificados (dependen del plan de su admin).
func RequireActiveSubscription(billingSvc BillingChecker) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            tipo := GetTipo(r)

            // ✅ SOLO aplicar a admins
            if tipo != "admin" {
                next.ServeHTTP(w, r)
                return
            }

            // Endpoints SIEMPRE permitidos para admins (sin importar suscripción)
            allowedPaths := []string{
                "/api/billing/plans",
                "/api/billing/metodos-pago",
                "/api/billing/precios/calcular",
                "/api/billing/empresa",
                "/api/billing/empresa/crear",
                "/api/billing/empresa/cambiar-plan",
                "/api/billing/empresa/conductores",
                "/api/billing/empresa/cancelar",
                "/api/billing/facturas",
                "/api/billing/historial",
                "/api/user/profile",
                "/api/user/profile/update",
                "/api/admin/registrar-conductor",  // Para que pueda registrar conductores
                "/api/admin/billing/empresas",
            }

            path := r.URL.Path
            for _, allowed := range allowedPaths {
                if path == allowed || strings.HasPrefix(path, allowed) {
                    next.ServeHTTP(w, r)
                    return
                }
            }

            // Verificar suscripción activa
            adminID := GetUserID(r)
            activa, err := billingSvc.IsSuscripcionActiva(adminID)
            if err != nil || !activa {
                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusForbidden)
                w.Write([]byte(`{
                    "error": "Completa tu plan para acceder a esta funcionalidad",
                    "code": 403,
                    "redirect": "/precios"
                }`))
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

type BillingChecker interface {
    IsSuscripcionActiva(adminID string) (bool, error)
}