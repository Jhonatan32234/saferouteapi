package billing

import (
	"time"
)

// ─── Planes ────────────────────────────────────────────────────

type Plan string

const (
	PlanBasico      Plan = "basico"
	PlanProfesional Plan = "profesional"
)

type EstadoSuscripcion string

const (
	EstadoTrial     EstadoSuscripcion = "trial"
	EstadoActivo    EstadoSuscripcion = "activo"
	EstadoPendiente EstadoSuscripcion = "pendiente"
	EstadoCancelado EstadoSuscripcion = "cancelado"
	EstadoExpirado  EstadoSuscripcion = "expirado"
)

type EstadoFactura string

const (
	FacturaPendiente   EstadoFactura = "pendiente"
	FacturaPagado      EstadoFactura = "pagado"
	FacturaCancelado   EstadoFactura = "cancelado"
	FacturaReembolsado EstadoFactura = "reembolsado"
	FacturaVencido     EstadoFactura = "vencido"
)

type EstadoPago string

const (
	PagoPendiente  EstadoPago = "pendiente"
	PagoCompletado EstadoPago = "completado"
	PagoFallido    EstadoPago = "fallido"
	PagoReembolsado EstadoPago = "reembolsado"
)

type ProveedorPago string

const (
	ProveedorStripe      ProveedorPago = "stripe"
	ProveedorMercadoPago ProveedorPago = "mercadopago"
	ProveedorPayPal      ProveedorPago = "paypal"
)

type TipoCambio string

const (
	CambioCreacion         TipoCambio = "creacion"
	CambioActivacion       TipoCambio = "activacion"
	CambioCambioPlan       TipoCambio = "cambio_plan"
	CambioAgregarConductor TipoCambio = "agregar_conductor"
	CambioRemoverConductor TipoCambio = "remover_conductor"
	CambioPagoRecibido     TipoCambio = "pago_recibido"
	CambioPagoFallido      TipoCambio = "pago_fallido"
	CambioCancelacion      TipoCambio = "cancelacion"
	CambioRenovacion       TipoCambio = "renovacion"
	CambioExpiracion       TipoCambio = "expiracion"
	CambioFacturaGenerada  TipoCambio = "factura_generada"
)

// ─── Precios ───────────────────────────────────────────────────

var PreciosPlanes = map[Plan]float64{
	PlanBasico:      2999.00,
	PlanProfesional: 5999.00,
}

var LimitesConductores = map[Plan]int{
	PlanBasico:      15,
	PlanProfesional: 30,
}

const PrecioConductorExtra = 199.00  // MXN/año por conductor extra
const IVA = 0.16                     // 16% IVA México

// ─── Estructuras ───────────────────────────────────────────────

type Empresa struct {
	ID                   string             `json:"id"`
	AdminID              string             `json:"admin_id"`
	NombreEmpresa        string             `json:"nombre_empresa"`
	RFC                  string             `json:"rfc,omitempty"`
	EmailFacturacion     string             `json:"email_facturacion,omitempty"`
	PlanActual           Plan               `json:"plan_actual"`
	StripeCustomerID     string             `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID string             `json:"stripe_subscription_id,omitempty"`
	EstadoSuscripcion    EstadoSuscripcion  `json:"estado_suscripcion"`
	TrialEndsAt          *time.Time         `json:"trial_ends_at,omitempty"`
	CurrentPeriodStart   *time.Time         `json:"current_period_start,omitempty"`
	CurrentPeriodEnd     *time.Time         `json:"current_period_end,omitempty"`
	MaxConductores       int                `json:"max_conductores"`
	ConductoresExtra     int                `json:"conductores_extra"`
	CreatedAt            time.Time          `json:"created_at"`
	UpdatedAt            time.Time          `json:"updated_at"`
}

type Factura struct {
	ID                    string     `json:"id"`
	EmpresaID             string     `json:"empresa_id"`
	StripeInvoiceID       string     `json:"stripe_invoice_id,omitempty"`
	StripePaymentIntentID string     `json:"stripe_payment_intent_id,omitempty"`
	Subtotal              float64    `json:"subtotal"`
	IVA                   float64    `json:"iva"`
	Total                 float64    `json:"total"`
	Plan                  Plan       `json:"plan"`
	ConductoresBase       int        `json:"conductores_base"`
	ConductoresExtra      int        `json:"conductores_extra"`
	CargoConductoresExtra float64    `json:"cargo_conductores_extra"`
	PeriodoInicio         *time.Time `json:"periodo_inicio,omitempty"`
	PeriodoFin            *time.Time `json:"periodo_fin,omitempty"`
	Estado                string     `json:"estado"`
	MetodoPago            string     `json:"metodo_pago,omitempty"`
	FechaEmision          time.Time  `json:"fecha_emision"`
	FechaPago             *time.Time `json:"fecha_pago,omitempty"`
	FechaVencimiento      *time.Time `json:"fecha_vencimiento,omitempty"`
	CFDIUUID              string     `json:"cfdi_uuid,omitempty"`
	CFDIXML               string     `json:"cfdi_xml,omitempty"`
	CFDIPDFURL            string     `json:"cfdi_pdf_url,omitempty"`
}

type Pago struct {
	ID                string          `json:"id"`
	FacturaID         string          `json:"factura_id"`
	EmpresaID         string          `json:"empresa_id"`
	Proveedor         ProveedorPago   `json:"proveedor"`
	ProviderPaymentID string          `json:"provider_payment_id,omitempty"`
	ProviderCheckoutID string         `json:"provider_checkout_id,omitempty"`
	Monto             float64         `json:"monto"`
	Moneda            string          `json:"moneda"`
	Estado            EstadoPago      `json:"estado"`
	Metadata          string          `json:"metadata,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type HistorialSuscripcion struct {
	ID          int64     `json:"id"`
	EmpresaID   string    `json:"empresa_id"`
	Cambio      string    `json:"cambio"`
	Descripcion string    `json:"descripcion,omitempty"`
	Metadata    string    `json:"metadata,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ─── Request/Response DTOs ─────────────────────────────────────

type CrearEmpresaRequest struct {
    NombreEmpresa    string `json:"nombre_empresa"`
    RFC              string `json:"rfc,omitempty"`
    EmailFacturacion string `json:"email_facturacion,omitempty"`
    Plan             Plan   `json:"plan"`
    MetodoPago       string `json:"metodo_pago"`
    ConductoresExtra int    `json:"conductores_extra"`
}

type CambiarPlanRequest struct {
	PlanNuevo        Plan `json:"plan_nuevo"`
	ConductoresExtra int  `json:"conductores_extra,omitempty"`
}

type AgregarConductoresRequest struct {
	Cantidad int `json:"cantidad"`
}

type CalcularPrecioResponse struct {
	Plan              Plan    `json:"plan"`
	ConductoresBase   int     `json:"conductores_base"`
	ConductoresExtra  int     `json:"conductores_extra"`
	CargoExtra        float64 `json:"cargo_extra"`
	Subtotal          float64 `json:"subtotal"`
	IVA               float64 `json:"iva"`
	Total             float64 `json:"total"`
	PrecioConductorExtra float64 `json:"precio_conductor_extra"`
}

type EmpresaResponse struct {
	ID                string             `json:"id"`
	NombreEmpresa     string             `json:"nombre_empresa"`
	RFC               string             `json:"rfc,omitempty"`
	PlanActual        Plan               `json:"plan_actual"`
	EstadoSuscripcion EstadoSuscripcion  `json:"estado_suscripcion"`
	MaxConductores    int                `json:"max_conductores"`
	ConductoresActuales int              `json:"conductores_actuales"`
	ConductoresExtra  int                `json:"conductores_extra"`
	PeriodoInicio     *time.Time         `json:"periodo_inicio,omitempty"`
	PeriodoFin        *time.Time         `json:"periodo_fin,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
}

type CheckoutResponse struct {
	Status      string `json:"status"`
	EmpresaID   string `json:"empresa_id"`
	CheckoutURL string `json:"checkout_url"`
	Total       float64 `json:"total"`
}

type PlanInfoResponse struct {
	Nombre              Plan    `json:"nombre"`
	Descripcion         string  `json:"descripcion"`
	PrecioAnual         float64 `json:"precio_anual"`
	LimiteConductores   int     `json:"limite_conductores"`
	PrecioConductorExtra float64 `json:"precio_conductor_extra"`
	Caracteristicas     []string `json:"caracteristicas"`
}

// ─── Funciones de cálculo ──────────────────────────────────────

func CalcularPrecioTotal(plan Plan, conductoresExtra int) (subtotal, iva, total float64) {
	precioBase := PreciosPlanes[plan]
	cargoExtra := float64(conductoresExtra) * PrecioConductorExtra
	subtotal = precioBase + cargoExtra
	iva = subtotal * IVA
	total = subtotal + iva
	return
}

func ObtenerInfoPlanes() []PlanInfoResponse {
	return []PlanInfoResponse{
		{
			Nombre:               PlanBasico,
			Descripcion:          "Plan Básico - Ideal para flotillas pequeñas",
			PrecioAnual:          PreciosPlanes[PlanBasico],
			LimiteConductores:    LimitesConductores[PlanBasico],
			PrecioConductorExtra: PrecioConductorExtra,
			Caracteristicas: []string{
				"15 conductores incluidos",
				"Monitoreo en tiempo real",
				"Reportes básicos de incidentes",
			},
		},
		{
			Nombre:               PlanProfesional,
			Descripcion:          "Plan Profesional - Para empresas en crecimiento",
			PrecioAnual:          PreciosPlanes[PlanProfesional],
			LimiteConductores:    LimitesConductores[PlanProfesional],
			PrecioConductorExtra: PrecioConductorExtra,
			Caracteristicas: []string{
				"30 conductores incluidos",
				"Monitoreo en tiempo real",
				"Reportes avanzados con IA",
				"Predicciones de zonas de riesgo",
				"Alertas personalizables",
			},
		},
	}
}