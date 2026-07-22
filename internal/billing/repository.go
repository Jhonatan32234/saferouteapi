package billing

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type Repository interface {
	// Empresas
	CrearEmpresa(e *Empresa) error
	GetEmpresaByID(id string) (*Empresa, error)
	GetEmpresaByAdminID(adminID string) (*Empresa, error)
	GetEmpresaByStripeCustomerID(customerID string) (*Empresa, error)
	UpdateEmpresa(e *Empresa) error
	UpdateSuscripcionStripe(id, subscriptionID string, periodStart, periodEnd time.Time) error
	ActivarSuscripcionConPlan(id, subscriptionID string, plan Plan, conductoresExtra int) error
	ActualizarEstadoSuscripcion(id string, estado EstadoSuscripcion) error
	ActualizarConductoresExtra(id string, extra int) error
	ListEmpresas() ([]*Empresa, error)
	GetTotalConductoresByEmpresa(empresaID string) (int, error)

	// Facturas
	CrearFactura(f *Factura) error
	GetFacturaByID(id string) (*Factura, error)
	GetFacturaByStripeInvoiceID(invoiceID string) (*Factura, error)
	ActualizarEstadoFactura(id string, estado EstadoFactura, fechaPago *time.Time, metodoPago string) error
	ListFacturasByEmpresa(empresaID string, limit, offset int) ([]*Factura, error)
	CountFacturasByEmpresa(empresaID string) (int, error)

	// Pagos
	CrearPago(p *Pago) error
	ActualizarEstadoPago(id string, estado EstadoPago, providerPaymentID string) error
	GetPagosByFactura(facturaID string) ([]*Pago, error)

	// Historial
	RegistrarHistorial(empresaID string, cambio TipoCambio, descripcion string, metadata map[string]interface{}) error
	ListHistorial(empresaID string, limit int) ([]*HistorialSuscripcion, error)
	GetEmpresaByStripeSubscriptionID(subID string) (*Empresa, error)

	UpdateFacturaStripeInvoiceID(facturaID, stripeInvoiceID string) error

}

type repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

// ─── Empresas ──────────────────────────────────────────────────

func (r *repository) GetEmpresaByStripeSubscriptionID(subID string) (*Empresa, error) {
    return r.scanEmpresa(r.db.QueryRow(`
        SELECT id, admin_id, nombre_empresa, COALESCE(rfc,''), COALESCE(email_facturacion,''),
               plan_actual, COALESCE(stripe_customer_id,''), COALESCE(stripe_subscription_id,''),
               estado_suscripcion, trial_ends_at, current_period_start, current_period_end,
               max_conductores, conductores_extra, created_at, updated_at
        FROM empresas WHERE stripe_subscription_id = $1`, subID))
}

func (r *repository) CrearEmpresa(e *Empresa) error {
	query := `
		INSERT INTO empresas (
			admin_id, nombre_empresa, rfc, email_facturacion,
			plan_actual, stripe_customer_id, stripe_subscription_id,
			estado_suscripcion, max_conductores, conductores_extra,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING id, created_at`

	return r.db.QueryRow(
		query,
		e.AdminID, e.NombreEmpresa, e.RFC, e.EmailFacturacion,
		e.PlanActual, e.StripeCustomerID, e.StripeSubscriptionID,
		e.EstadoSuscripcion, e.MaxConductores, e.ConductoresExtra,
	).Scan(&e.ID, &e.CreatedAt)
}

func (r *repository) GetEmpresaByID(id string) (*Empresa, error) {
	return r.scanEmpresa(r.db.QueryRow(`
		SELECT id, admin_id, nombre_empresa, COALESCE(rfc,''), COALESCE(email_facturacion,''),
			   plan_actual, COALESCE(stripe_customer_id,''), COALESCE(stripe_subscription_id,''),
			   estado_suscripcion, trial_ends_at, current_period_start, current_period_end,
			   max_conductores, conductores_extra, created_at, updated_at
		FROM empresas WHERE id = $1`, id))
}

func (r *repository) GetEmpresaByAdminID(adminID string) (*Empresa, error) {
	return r.scanEmpresa(r.db.QueryRow(`
		SELECT id, admin_id, nombre_empresa, COALESCE(rfc,''), COALESCE(email_facturacion,''),
			   plan_actual, COALESCE(stripe_customer_id,''), COALESCE(stripe_subscription_id,''),
			   estado_suscripcion, trial_ends_at, current_period_start, current_period_end,
			   max_conductores, conductores_extra, created_at, updated_at
		FROM empresas WHERE admin_id = $1`, adminID))
}

func (r *repository) GetEmpresaByStripeCustomerID(customerID string) (*Empresa, error) {
	return r.scanEmpresa(r.db.QueryRow(`
		SELECT id, admin_id, nombre_empresa, COALESCE(rfc,''), COALESCE(email_facturacion,''),
			   plan_actual, COALESCE(stripe_customer_id,''), COALESCE(stripe_subscription_id,''),
			   estado_suscripcion, trial_ends_at, current_period_start, current_period_end,
			   max_conductores, conductores_extra, created_at, updated_at
		FROM empresas WHERE stripe_customer_id = $1`, customerID))
}


func (r *repository) UpdateFacturaStripeInvoiceID(facturaID, stripeInvoiceID string) error {
    _, err := r.db.Exec(`
        UPDATE facturas SET stripe_invoice_id = $1 WHERE id = $2`,
        stripeInvoiceID, facturaID)
    return err
}


// billing/repository.go
func (r *repository) UpdateEmpresa(e *Empresa) error {
    _, err := r.db.Exec(`
        UPDATE empresas SET
            nombre_empresa = $1, rfc = $2, email_facturacion = $3,
            plan_actual = $4, max_conductores = $5,
            stripe_customer_id = $6,
            updated_at = NOW()
        WHERE id = $7`,
        e.NombreEmpresa, e.RFC, e.EmailFacturacion,
        e.PlanActual, e.MaxConductores, e.StripeCustomerID, e.ID)
    return err
}

func (r *repository) UpdateSuscripcionStripe(id, subscriptionID string, periodStart, periodEnd time.Time) error {
	_, err := r.db.Exec(`
		UPDATE empresas SET
			stripe_subscription_id = $1,
			current_period_start = $2,
			current_period_end = $3,
			updated_at = NOW()
		WHERE id = $4`,
		subscriptionID, periodStart, periodEnd, id)
	return err
}

// billing/repository.go

// ActivarSuscripcionConPlan - CORREGIDO
func (r *repository) ActivarSuscripcionConPlan(id, subscriptionID string, plan Plan, conductoresExtra int) error {
    now := time.Now()
    yearEnd := now.AddDate(1, 0, 0)
    maxConductores := LimitesConductores[plan]
    
    _, err := r.db.Exec(`
        UPDATE empresas SET
            estado_suscripcion = 'activo',
            stripe_subscription_id = $1,
            plan_actual = $2,
            max_conductores = $3,
            conductores_extra = $4,
            current_period_start = $5,
            current_period_end = $6,
            updated_at = NOW()
        WHERE id = $7`,
        subscriptionID, plan, maxConductores, conductoresExtra, now, yearEnd, id)
    return err
}

func (r *repository) ActualizarEstadoSuscripcion(id string, estado EstadoSuscripcion) error {
	_, err := r.db.Exec(`
		UPDATE empresas SET estado_suscripcion = $1, updated_at = NOW()
		WHERE id = $2`, estado, id)
	return err
}

func (r *repository) ActualizarConductoresExtra(id string, extra int) error {
	_, err := r.db.Exec(`
		UPDATE empresas SET conductores_extra = $1, updated_at = NOW()
		WHERE id = $2`, extra, id)
	return err
}

func (r *repository) ListEmpresas() ([]*Empresa, error) {
	rows, err := r.db.Query(`
		SELECT id, admin_id, nombre_empresa, COALESCE(rfc,''), COALESCE(email_facturacion,''),
			   plan_actual, COALESCE(stripe_customer_id,''), COALESCE(stripe_subscription_id,''),
			   estado_suscripcion, trial_ends_at, current_period_start, current_period_end,
			   max_conductores, conductores_extra, created_at, updated_at
		FROM empresas ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var empresas []*Empresa
	for rows.Next() {
		e, err := r.scanEmpresaFromRow(rows)
		if err != nil {
			log.Printf("Error escaneando empresa: %v", err)
			continue
		}
		empresas = append(empresas, e)
	}
	return empresas, nil
}

func (r *repository) GetTotalConductoresByEmpresa(empresaID string) (int, error) {
    var total int
    err := r.db.QueryRow(`
        SELECT COUNT(*) FROM usuarios 
        WHERE empresa_id = $1 AND tipo = 'conductor'`, empresaID).Scan(&total)
    return total, err
}

// ─── Facturas ──────────────────────────────────────────────────

func (r *repository) CrearFactura(f *Factura) error {
    query := `
        INSERT INTO facturas (
            empresa_id, stripe_invoice_id, stripe_payment_intent_id,
            subtotal, iva, total, plan,
            conductores_base, conductores_extra, cargo_conductores_extra,
            periodo_inicio, periodo_fin, estado, metodo_pago,
            fecha_emision, fecha_vencimiento
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW(), $15)
        RETURNING id, fecha_emision`

    fechaVenc := time.Now().Add(30 * 24 * time.Hour)
    
    err := r.db.QueryRow(
        query,
        f.EmpresaID, 
        f.StripeInvoiceID, 
        f.StripePaymentIntentID,
        f.Subtotal, 
        f.IVA, 
        f.Total, 
        f.Plan,
        f.ConductoresBase, 
        f.ConductoresExtra, 
        f.CargoConductoresExtra,
        f.PeriodoInicio,   // Puede ser nil → NULL
        f.PeriodoFin,      // Puede ser nil → NULL
        f.Estado, 
        f.MetodoPago,
        fechaVenc,
    ).Scan(&f.ID, &f.FechaEmision)
    
    if err != nil {
        log.Printf("[FACTURA] Error creando factura: %v | Datos: empresa=%s plan=%s subtotal=%.2f estado=%s", 
            err, f.EmpresaID, f.Plan, f.Subtotal, f.Estado)
        return err
    }
    
    log.Printf("[FACTURA] Creada: %s - %s $%.2f", f.ID, f.Plan, f.Total)
    return nil
}

func (r *repository) GetFacturaByID(id string) (*Factura, error) {
	return r.scanFactura(r.db.QueryRow(`
		SELECT id, empresa_id, COALESCE(stripe_invoice_id,''), COALESCE(stripe_payment_intent_id,''),
			   subtotal, iva, total, plan,
			   conductores_base, conductores_extra, cargo_conductores_extra,
			   periodo_inicio, periodo_fin, estado, metodo_pago,
			   fecha_emision, fecha_pago, fecha_vencimiento,
			   COALESCE(cfdi_uuid,''), COALESCE(cfdi_xml,''), COALESCE(cfdi_pdf_url,'')
		FROM facturas WHERE id = $1`, id))
}

func (r *repository) GetFacturaByStripeInvoiceID(invoiceID string) (*Factura, error) {
	return r.scanFactura(r.db.QueryRow(`
		SELECT id, empresa_id, COALESCE(stripe_invoice_id,''), COALESCE(stripe_payment_intent_id,''),
			   subtotal, iva, total, plan,
			   conductores_base, conductores_extra, cargo_conductores_extra,
			   periodo_inicio, periodo_fin, estado, metodo_pago,
			   fecha_emision, fecha_pago, fecha_vencimiento,
			   COALESCE(cfdi_uuid,''), COALESCE(cfdi_xml,''), COALESCE(cfdi_pdf_url,'')
		FROM facturas WHERE stripe_invoice_id = $1`, invoiceID))
}

func (r *repository) ActualizarEstadoFactura(id string, estado EstadoFactura, fechaPago *time.Time, metodoPago string) error {
    if fechaPago != nil {
        _, err := r.db.Exec(`
            UPDATE facturas SET estado = $1, fecha_pago = $2, metodo_pago = $3
            WHERE id = $4`, estado, *fechaPago, metodoPago, id)
        return err
    }
    _, err := r.db.Exec(`
        UPDATE facturas SET estado = $1, metodo_pago = $2
        WHERE id = $3`, estado, metodoPago, id)
    return err
}


func (r *repository) ListFacturasByEmpresa(empresaID string, limit, offset int) ([]*Factura, error) {
	rows, err := r.db.Query(`
		SELECT id, empresa_id, COALESCE(stripe_invoice_id,''), COALESCE(stripe_payment_intent_id,''),
			   subtotal, iva, total, plan,
			   conductores_base, conductores_extra, cargo_conductores_extra,
			   periodo_inicio, periodo_fin, estado, metodo_pago,
			   fecha_emision, fecha_pago, fecha_vencimiento,
			   COALESCE(cfdi_uuid,''), COALESCE(cfdi_xml,''), COALESCE(cfdi_pdf_url,'')
		FROM facturas WHERE empresa_id = $1
		ORDER BY fecha_emision DESC LIMIT $2 OFFSET $3`,
		empresaID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var facturas []*Factura
	for rows.Next() {
		f, err := r.scanFacturaFromRow(rows)
		if err != nil {
			continue
		}
		facturas = append(facturas, f)
	}
	return facturas, nil
}

func (r *repository) CountFacturasByEmpresa(empresaID string) (int, error) {
	var total int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM facturas WHERE empresa_id = $1`, empresaID).Scan(&total)
	return total, err
}

// ─── Pagos ─────────────────────────────────────────────────────

func (r *repository) CrearPago(p *Pago) error {
	query := `
		INSERT INTO pagos (factura_id, empresa_id, proveedor, provider_payment_id,
			provider_checkout_id, monto, moneda, estado, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`

	metaJSON := "{}"
	if p.Metadata != "" {
		metaJSON = p.Metadata
	}

	return r.db.QueryRow(query,
		p.FacturaID, p.EmpresaID, p.Proveedor,
		p.ProviderPaymentID, p.ProviderCheckoutID,
		p.Monto, p.Moneda, p.Estado, metaJSON,
	).Scan(&p.ID, &p.CreatedAt)
}

func (r *repository) ActualizarEstadoPago(id string, estado EstadoPago, providerPaymentID string) error {
	_, err := r.db.Exec(`
		UPDATE pagos SET estado = $1, provider_payment_id = $2, updated_at = NOW()
		WHERE id = $3`, estado, providerPaymentID, id)
	return err
}

func (r *repository) GetPagosByFactura(facturaID string) ([]*Pago, error) {
	rows, err := r.db.Query(`
		SELECT id, factura_id, empresa_id, proveedor,
			   COALESCE(provider_payment_id,''), COALESCE(provider_checkout_id,''),
			   monto, moneda, estado, metadata::text, created_at, updated_at
		FROM pagos WHERE factura_id = $1 ORDER BY created_at DESC`, facturaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pagos []*Pago
	for rows.Next() {
		p := &Pago{}
		err := rows.Scan(&p.ID, &p.FacturaID, &p.EmpresaID, &p.Proveedor,
			&p.ProviderPaymentID, &p.ProviderCheckoutID,
			&p.Monto, &p.Moneda, &p.Estado, &p.Metadata,
			&p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			continue
		}
		pagos = append(pagos, p)
	}
	return pagos, nil
}

// ─── Historial ─────────────────────────────────────────────────

func (r *repository) RegistrarHistorial(empresaID string, cambio TipoCambio, descripcion string, metadata map[string]interface{}) error {
	metaJSON := "{}"
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err == nil {
			metaJSON = string(b)
		}
	}

	_, err := r.db.Exec(`
		INSERT INTO historial_suscripcion (empresa_id, cambio, descripcion, metadata)
		VALUES ($1, $2, $3, $4)`,
		empresaID, string(cambio), descripcion, metaJSON)
	return err
}

func (r *repository) ListHistorial(empresaID string, limit int) ([]*HistorialSuscripcion, error) {
	rows, err := r.db.Query(`
		SELECT id, empresa_id, cambio, COALESCE(descripcion,''),
			   COALESCE(metadata::text,'{}'), created_at
		FROM historial_suscripcion
		WHERE empresa_id = $1
		ORDER BY created_at DESC LIMIT $2`,
		empresaID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var historial []*HistorialSuscripcion
	for rows.Next() {
		h := &HistorialSuscripcion{}
		err := rows.Scan(&h.ID, &h.EmpresaID, &h.Cambio, &h.Descripcion, &h.Metadata, &h.CreatedAt)
		if err != nil {
			continue
		}
		historial = append(historial, h)
	}
	return historial, nil
}

// ─── Scanners ──────────────────────────────────────────────────

func (r *repository) scanEmpresa(row *sql.Row) (*Empresa, error) {
	e := &Empresa{}
	var trialEndsAt, periodStart, periodEnd sql.NullTime

	err := row.Scan(
		&e.ID, &e.AdminID, &e.NombreEmpresa, &e.RFC, &e.EmailFacturacion,
		&e.PlanActual, &e.StripeCustomerID, &e.StripeSubscriptionID,
		&e.EstadoSuscripcion, &trialEndsAt, &periodStart, &periodEnd,
		&e.MaxConductores, &e.ConductoresExtra, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if trialEndsAt.Valid {
		e.TrialEndsAt = &trialEndsAt.Time
	}
	if periodStart.Valid {
		e.CurrentPeriodStart = &periodStart.Time
	}
	if periodEnd.Valid {
		e.CurrentPeriodEnd = &periodEnd.Time
	}

	return e, nil
}

func (r *repository) scanEmpresaFromRow(rows *sql.Rows) (*Empresa, error) {
	e := &Empresa{}
	var trialEndsAt, periodStart, periodEnd sql.NullTime

	err := rows.Scan(
		&e.ID, &e.AdminID, &e.NombreEmpresa, &e.RFC, &e.EmailFacturacion,
		&e.PlanActual, &e.StripeCustomerID, &e.StripeSubscriptionID,
		&e.EstadoSuscripcion, &trialEndsAt, &periodStart, &periodEnd,
		&e.MaxConductores, &e.ConductoresExtra, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan empresa: %w", err)
	}

	if trialEndsAt.Valid {
		e.TrialEndsAt = &trialEndsAt.Time
	}
	if periodStart.Valid {
		e.CurrentPeriodStart = &periodStart.Time
	}
	if periodEnd.Valid {
		e.CurrentPeriodEnd = &periodEnd.Time
	}

	return e, nil
}

func (r *repository) scanFactura(row *sql.Row) (*Factura, error) {
	f := &Factura{}
	var periodoInicio, periodoFin, fechaPago, fechaVencimiento sql.NullTime

	err := row.Scan(
		&f.ID, &f.EmpresaID, &f.StripeInvoiceID, &f.StripePaymentIntentID,
		&f.Subtotal, &f.IVA, &f.Total, &f.Plan,
		&f.ConductoresBase, &f.ConductoresExtra, &f.CargoConductoresExtra,
		&periodoInicio, &periodoFin, &f.Estado, &f.MetodoPago,
		&f.FechaEmision, &fechaPago, &fechaVencimiento,
		&f.CFDIUUID, &f.CFDIXML, &f.CFDIPDFURL,
	)
	if err != nil {
		return nil, err
	}

	if periodoInicio.Valid {
		f.PeriodoInicio = &periodoInicio.Time
	}
	if periodoFin.Valid {
		f.PeriodoFin = &periodoFin.Time
	}
	if fechaPago.Valid {
		f.FechaPago = &fechaPago.Time
	}
	if fechaVencimiento.Valid {
		f.FechaVencimiento = &fechaVencimiento.Time
	}

	return f, nil
}

func (r *repository) scanFacturaFromRow(rows *sql.Rows) (*Factura, error) {
	f := &Factura{}
	var periodoInicio, periodoFin, fechaPago, fechaVencimiento sql.NullTime

	err := rows.Scan(
		&f.ID, &f.EmpresaID, &f.StripeInvoiceID, &f.StripePaymentIntentID,
		&f.Subtotal, &f.IVA, &f.Total, &f.Plan,
		&f.ConductoresBase, &f.ConductoresExtra, &f.CargoConductoresExtra,
		&periodoInicio, &periodoFin, &f.Estado, &f.MetodoPago,
		&f.FechaEmision, &fechaPago, &fechaVencimiento,
		&f.CFDIUUID, &f.CFDIXML, &f.CFDIPDFURL,
	)
	if err != nil {
		return nil, fmt.Errorf("scan factura: %w", err)
	}

	if periodoInicio.Valid {
		f.PeriodoInicio = &periodoInicio.Time
	}
	if periodoFin.Valid {
		f.PeriodoFin = &periodoFin.Time
	}
	if fechaPago.Valid {
		f.FechaPago = &fechaPago.Time
	}
	if fechaVencimiento.Valid {
		f.FechaVencimiento = &fechaVencimiento.Time
	}

	return f, nil
}