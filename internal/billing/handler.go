package billing

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"saferoute/internal/common"
	"saferoute/internal/middleware"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ─── Planes ────────────────────────────────────────────────────

// GetPlanesHandler GET /api/billing/plans
func (h *Handler) GetPlanesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		planes := ObtenerInfoPlanes()
		common.WriteJSON(w, http.StatusOK, planes)
	}
}

// ─── Suscripción ───────────────────────────────────────────────

// CrearSuscripcionHandler POST /api/billing/empresa/crear
func (h *Handler) CrearSuscripcionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID := middleware.GetUserID(r)
		if adminID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		var req CrearEmpresaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos inválidos")
			return
		}

		// Validar campos requeridos
		if req.NombreEmpresa == "" {
			common.WriteError(w, http.StatusBadRequest, "nombre_empresa es requerido")
			return
		}
		if req.Plan == "" {
			common.WriteError(w, http.StatusBadRequest, "plan es requerido (basico/profesional)")
			return
		}
		if req.MetodoPago == "" {
			common.WriteError(w, http.StatusBadRequest, "metodo_pago es requerido (tarjeta/oxxo/spei)")
			return
		}

		result, err := h.svc.CrearEmpresa(adminID, req)
		if err != nil {
			common.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		common.WriteJSON(w, http.StatusCreated, result)
	}
}



// GetMiEmpresaHandler GET /api/billing/empresa
func (h *Handler) GetMiEmpresaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID := middleware.GetUserID(r)
		if adminID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		empresa, err := h.svc.GetMiEmpresa(adminID)
		if err != nil {
			common.WriteError(w, http.StatusNotFound, err.Error())
			return
		}

		common.WriteJSON(w, http.StatusOK, empresa)
	}
}

// CambiarPlanHandler PUT /api/billing/empresa/cambiar-plan
func (h *Handler) CambiarPlanHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID := middleware.GetUserID(r)
		if adminID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		// Verificar que sea admin
		tipo := middleware.GetTipo(r)
		if tipo != "admin" {
			common.WriteError(w, http.StatusForbidden, "solo administradores pueden cambiar el plan")
			return
		}

		var req CambiarPlanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos inválidos")
			return
		}

		if req.PlanNuevo != PlanBasico && req.PlanNuevo != PlanProfesional {
			common.WriteError(w, http.StatusBadRequest, "plan inválido: use 'basico' o 'profesional'")
			return
		}

		if err := h.svc.CambiarPlan(adminID, req); err != nil {
			common.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		common.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "plan actualizado",
		})
	}
}

// AgregarConductoresHandler POST /api/billing/empresa/conductores
func (h *Handler) AgregarConductoresHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID := middleware.GetUserID(r)
		tipo := middleware.GetTipo(r)

		if tipo != "admin" {
            common.WriteError(w, http.StatusForbidden, "solo administradores pueden modificar conductores")
            return
        }

		if adminID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		var req AgregarConductoresRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos inválidos")
			return
		}

		if req.Cantidad <= 0 {
			common.WriteError(w, http.StatusBadRequest, "la cantidad debe ser mayor a 0")
			return
		}

		if err := h.svc.AgregarConductores(adminID, req.Cantidad); err != nil {
			common.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		common.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "conductores agregados",
		})
	}
}

// CancelarSuscripcionHandler POST /api/billing/empresa/cancelar
func (h *Handler) CancelarSuscripcionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID := middleware.GetUserID(r)
		tipo := middleware.GetTipo(r)

		if tipo != "admin" {
            common.WriteError(w, http.StatusForbidden, "solo administradores pueden cancelar la suscripción")
            return
        }

		if adminID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		if err := h.svc.CancelarSuscripcion(adminID); err != nil {
			common.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		common.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "suscripción cancelada",
		})
	}
}

// ─── Facturas ──────────────────────────────────────────────────

// GetFacturasHandler GET /api/billing/facturas?page=1&limit=10
func (h *Handler) GetFacturasHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID := middleware.GetUserID(r)
		if adminID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit < 1 || limit > 50 {
			limit = 10
		}

		facturas, total, err := h.svc.ListFacturas(adminID, page, limit)
		if err != nil {
			common.WriteError(w, http.StatusNotFound, err.Error())
			return
		}

		totalPaginas := (total + limit - 1) / limit
		if totalPaginas == 0 {
			totalPaginas = 1
		}

		common.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"facturas":     facturas,
			"total":        total,
			"pagina":       page,
			"total_paginas": totalPaginas,
		})
	}
}

// ─── Historial ─────────────────────────────────────────────────

// GetHistorialHandler GET /api/billing/historial?limit=20
func (h *Handler) GetHistorialHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID := middleware.GetUserID(r)
		if adminID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit < 1 || limit > 100 {
			limit = 20
		}

		historial, err := h.svc.ListHistorial(adminID, limit)
		if err != nil {
			common.WriteError(w, http.StatusNotFound, err.Error())
			return
		}

		common.WriteJSON(w, http.StatusOK, historial)
	}
}

// ─── Precios ───────────────────────────────────────────────────

// CalcularPrecioHandler GET /api/billing/precios/calcular?plan=basico&extra=5
func (h *Handler) CalcularPrecioHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		plan := Plan(r.URL.Query().Get("plan"))
		extra, _ := strconv.Atoi(r.URL.Query().Get("extra"))

		if plan != PlanBasico && plan != PlanProfesional {
			common.WriteError(w, http.StatusBadRequest, "plan inválido: use 'basico' o 'profesional'")
			return
		}
		if extra < 0 {
			extra = 0
		}

		result := h.svc.CalcularPrecio(plan, extra)
		common.WriteJSON(w, http.StatusOK, result)
	}
}

// ─── Webhook ───────────────────────────────────────────────────

// WebhookStripeHandler POST /api/webhooks/stripe
func (h *Handler) WebhookStripeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error leyendo webhook body: %v", err)
			common.WriteError(w, http.StatusBadRequest, "error leyendo body")
			return
		}

		signature := r.Header.Get("Stripe-Signature")
		if signature == "" {
			common.WriteError(w, http.StatusBadRequest, "Stripe-Signature header requerido")
			return
		}

		if err := h.svc.ProcesarWebhook(payload, signature); err != nil {
			log.Printf("Error procesando webhook: %v", err)
			common.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		common.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// ─── Admin ─────────────────────────────────────────────────────

// AdminListEmpresasHandler GET /api/admin/billing/empresas
func (h *Handler) AdminListEmpresasHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		empresas, err := h.svc.ListAllEmpresas()
		if err != nil {
			common.WriteError(w, http.StatusInternalServerError, "error listando empresas")
			return
		}

		common.WriteJSON(w, http.StatusOK, empresas)
	}
}

// ─── Método de pago info ───────────────────────────────────────

// GetMetodosPagoHandler GET /api/billing/metodos-pago
func (h *Handler) GetMetodosPagoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metodos := []map[string]interface{}{
			{
				"id":          "tarjeta",
				"nombre":      "Tarjeta de crédito/débito",
				"proveedor":   "Stripe",
				"comision":    "2.9% + $3 MXN",
			},
			{
				"id":          "oxxo",
				"nombre":      "Pago en OXXO",
				"proveedor":   "Stripe",
				"comision":    "1.5%",
				"instrucciones": "Genera tu código de pago y paga en cualquier OXXO",
			},
			{
				"id":          "spei",
				"nombre":      "Transferencia SPEI",
				"proveedor":   "Stripe",
				"comision":    "1.5%",
				"instrucciones": "Transferencia bancaria instantánea",
			},
		}
		common.WriteJSON(w, http.StatusOK, metodos)
	}
}