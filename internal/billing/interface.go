package billing

// BillingService define la interfaz que deben implementar tanto el servicio local
// como el cliente HTTP para el billing service independiente.
type BillingService interface {
	GetEmpresaByAdminID(adminID string) (*Empresa, error)
	GetTotalConductoresByEmpresa(empresaID string) (int, error)
	CrearEmpresa(adminID string, req CrearEmpresaRequest) (*CheckoutResponse, error)
	GetMiEmpresa(adminID string) (*EmpresaResponse, error)
	CambiarPlan(adminID string, req CambiarPlanRequest) error
	QuitarConductores(adminID string, cantidad int) error
	AgregarConductores(adminID string, cantidad int) error
	CancelarSuscripcion(adminID string) error
	IsSuscripcionActiva(adminID string) (bool, error)
	ListFacturas(adminID string, page, limit int) ([]*Factura, int, error)
	ListHistorial(adminID string, limit int) ([]*HistorialSuscripcion, error)
	ListAllEmpresas() ([]*Empresa, error)
	CalcularPrecio(plan Plan, conductoresExtra int) CalcularPrecioResponse
	ProcesarWebhook(payload []byte, signature string) error
}