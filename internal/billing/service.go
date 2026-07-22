package billing

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/customer"
	"github.com/stripe/stripe-go/v81/subscription" // ← AGREGAR
	"github.com/stripe/stripe-go/v81/subscriptionitem"
	"github.com/stripe/stripe-go/v81/webhook"
)

type StripeConfig struct {
	SecretKey      string
	WebhookSecret  string
	PriceBasico    string
	PricePro       string
	PriceExtra     string
	SuccessURL     string
	CancelURL      string
}

type Service struct {
	repo     Repository
	stripeCfg *StripeConfig
}

func NewService(repo Repository, stripeCfg *StripeConfig) *Service {
	if stripeCfg != nil && stripeCfg.SecretKey != "" {
		stripe.Key = stripeCfg.SecretKey
	}
	return &Service{
		repo:      repo,
		stripeCfg: stripeCfg,
	}
}

func (s *Service) GetEmpresaByAdminID(adminID string) (*Empresa, error) {
    return s.repo.GetEmpresaByAdminID(adminID)
}

func (s *Service) GetTotalConductoresByEmpresa(empresaID string) (int, error) {
    return s.repo.GetTotalConductoresByEmpresa(empresaID)
}

// ─── Empresa ───────────────────────────────────────────────────

func (s *Service) CrearEmpresa(adminID string, req CrearEmpresaRequest) (*CheckoutResponse, error) {
    // Validar plan
    if req.Plan != PlanBasico && req.Plan != PlanProfesional {
        return nil, fmt.Errorf("plan inválido: debe ser 'basico' o 'profesional'")
    }

    // Validar método de pago
    metodosValidos := map[string]bool{
        "tarjeta": true,
        "oxxo":    true,
        "spei":    true,
    }
    if !metodosValidos[req.MetodoPago] {
        return nil, fmt.Errorf("método de pago inválido: use 'tarjeta', 'oxxo' o 'spei'")
    }

    // Verificar si ya existe una empresa para este admin
    existing, _ := s.repo.GetEmpresaByAdminID(adminID)
    
    var empresa *Empresa
    
    if existing != nil {
        // ✅ Si la empresa existe pero está pendiente, actualizarla
        if existing.EstadoSuscripcion == EstadoPendiente {
            existing.NombreEmpresa = req.NombreEmpresa
            existing.PlanActual = req.Plan
            existing.MaxConductores = LimitesConductores[req.Plan]
            existing.RFC = req.RFC
            existing.EmailFacturacion = req.EmailFacturacion
            
            if err := s.repo.UpdateEmpresa(existing); err != nil {
                return nil, fmt.Errorf("error actualizando empresa: %w", err)
            }
            
            empresa = existing
            log.Printf("[BILLING] Empresa pendiente actualizada: %s con plan %s", empresa.ID, req.Plan)
            
        } else if existing.EstadoSuscripcion == EstadoActivo {
            return nil, fmt.Errorf("el administrador ya tiene una empresa activa. Si deseas cambiar de plan, usa la opción 'Cambiar Plan'")
            
        } else {
            // Cancelada o expirada → permitir reactivar
            existing.NombreEmpresa = req.NombreEmpresa
            existing.PlanActual = req.Plan
            existing.MaxConductores = LimitesConductores[req.Plan]
            existing.EstadoSuscripcion = EstadoPendiente
            existing.RFC = req.RFC
            existing.EmailFacturacion = req.EmailFacturacion
            
            if err := s.repo.UpdateEmpresa(existing); err != nil {
                return nil, fmt.Errorf("error actualizando empresa: %w", err)
            }
            
            empresa = existing
            log.Printf("[BILLING] Empresa %s reactivada con plan %s", empresa.ID, req.Plan)
        }
    } else {
        // ✅ Crear nueva empresa (primera vez)
        empresa = &Empresa{
            AdminID:           adminID,
            NombreEmpresa:     req.NombreEmpresa,
            RFC:               req.RFC,
            EmailFacturacion:  req.EmailFacturacion,
            PlanActual:        req.Plan,
            EstadoSuscripcion: EstadoPendiente,
            MaxConductores:    LimitesConductores[req.Plan],
            ConductoresExtra:  0,
        }
        
        if err := s.repo.CrearEmpresa(empresa); err != nil {
            return nil, fmt.Errorf("error guardando empresa: %w", err)
        }
        
        log.Printf("[BILLING] Nueva empresa creada: %s con plan %s", empresa.ID, req.Plan)
    }

    // ✅ Crear customer en Stripe si no tiene uno
    if s.stripeCfg != nil && s.stripeCfg.SecretKey != "" && empresa.StripeCustomerID == "" {
        cust, err := customer.New(&stripe.CustomerParams{
            Name:  stripe.String(empresa.NombreEmpresa),
            Email: stripe.String(empresa.EmailFacturacion),
            Metadata: map[string]string{
                "admin_id":        adminID,
                "nombre_empresa":  empresa.NombreEmpresa,
            },
        })
        if err != nil {
            log.Printf("[BILLING] Error creando customer en Stripe: %v", err)
            // No fallar - se puede crear después
        } else {
            empresa.StripeCustomerID = cust.ID
            // Actualizar el customer ID en BD
            _ = s.repo.UpdateEmpresa(empresa)
        }
    }

    // Calcular precios
    subtotal, iva, total := CalcularPrecioTotal(req.Plan, 0)

    // Registrar historial
    _ = s.repo.RegistrarHistorial(empresa.ID, CambioCreacion,
        fmt.Sprintf("Empresa creada con plan %s", req.Plan), map[string]interface{}{
            "plan":        req.Plan,
            "metodo_pago": req.MetodoPago,
        })

    // Crear factura pendiente
    now := time.Now()
    yearEnd := now.AddDate(1, 0, 0)
    factura := &Factura{
        EmpresaID:             empresa.ID,
        Subtotal:              subtotal,
        IVA:                   iva,
        Total:                 total,
        Plan:                  req.Plan,
        ConductoresBase:       LimitesConductores[req.Plan],
        ConductoresExtra:      0,
        CargoConductoresExtra: 0,
        PeriodoInicio:         &now,
        PeriodoFin:            &yearEnd,
        Estado:                string(FacturaPendiente),
        MetodoPago:            req.MetodoPago,
    }
    if err := s.repo.CrearFactura(factura); err != nil {
        log.Printf("[BILLING] Error creando factura: %v", err)
    }

    _ = s.repo.RegistrarHistorial(empresa.ID, CambioFacturaGenerada,
        fmt.Sprintf("Factura generada: $%.2f MXN", total), nil)

    // Crear sesión de checkout en Stripe
    checkoutURL := ""
    if s.stripeCfg != nil && s.stripeCfg.SecretKey != "" {
        url, err := s.crearCheckoutSession(empresa, factura, req.MetodoPago)
        if err != nil {
            log.Printf("[BILLING] Error creando checkout session: %v", err)
        } else {
            checkoutURL = url
        }
    }

    return &CheckoutResponse{
        Status:      "pendiente",
        EmpresaID:   empresa.ID,
        CheckoutURL: checkoutURL,
        Total:       total,
    }, nil
}

func (s *Service) crearCheckoutSession(empresa *Empresa, factura *Factura, metodoPago string) (string, error) {
	var priceID string
	switch empresa.PlanActual {
	case PlanBasico:
		priceID = s.stripeCfg.PriceBasico
	case PlanProfesional:
		priceID = s.stripeCfg.PricePro
	}

	if priceID == "" {
		return "", fmt.Errorf("price_id no configurado para el plan %s", empresa.PlanActual)
	}

	// Mapear método de pago a tipos de Stripe
	var paymentMethodTypes []string
	switch metodoPago {
	case "tarjeta":
		paymentMethodTypes = []string{"card"}
	case "oxxo":
		paymentMethodTypes = []string{"oxxo"}
	case "spei":
		paymentMethodTypes = []string{"customer_balance"}
	default:
		paymentMethodTypes = []string{"card", "oxxo"}
	}

	lineItems := []*stripe.CheckoutSessionLineItemParams{
		{
			Price:    stripe.String(priceID),
			Quantity: stripe.Int64(1),
		},
	}

	// Agregar conductores extra si aplica
	if empresa.ConductoresExtra > 0 && s.stripeCfg.PriceExtra != "" {
		lineItems = append(lineItems, &stripe.CheckoutSessionLineItemParams{
			Price:    stripe.String(s.stripeCfg.PriceExtra),
			Quantity: stripe.Int64(int64(empresa.ConductoresExtra)),
		})
	}

	successURL := s.stripeCfg.SuccessURL
	if successURL == "" {
		successURL = "https://saferoute.mx/dashboard?pago=exitoso"
	}
	cancelURL := s.stripeCfg.CancelURL
	if cancelURL == "" {
		cancelURL = "https://saferoute.mx/dashboard?pago=cancelado"
	}

	params := &stripe.CheckoutSessionParams{
		Customer:          stripe.String(empresa.StripeCustomerID),
		Mode:              stripe.String("subscription"),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		LineItems:         lineItems,
		PaymentMethodTypes: stripe.StringSlice(paymentMethodTypes),
		Metadata: map[string]string{
			"empresa_id": empresa.ID,
			"factura_id": factura.ID,
			"plan":       string(empresa.PlanActual),
		},
	}

	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("error creando checkout session: %w", err)
	}

	// Guardar referencia del checkout en el pago
	pago := &Pago{
		FacturaID:        factura.ID,
		EmpresaID:        empresa.ID,
		Proveedor:        ProveedorStripe,
		ProviderCheckoutID: sess.ID,
		Monto:            factura.Total,
		Moneda:           "MXN",
		Estado:           PagoPendiente,
	}
	if err := s.repo.CrearPago(pago); err != nil {
		log.Printf("Error guardando pago: %v", err)
	}

	return sess.URL, nil
}

// ─── Obtener empresa ───────────────────────────────────────────

func (s *Service) GetMiEmpresa(adminID string) (*EmpresaResponse, error) {
	empresa, err := s.repo.GetEmpresaByAdminID(adminID)
	if err != nil {
		return nil, fmt.Errorf("empresa no encontrada")
	}

	totalConductores, _ := s.repo.GetTotalConductoresByEmpresa(empresa.ID)

	return &EmpresaResponse{
		ID:                 empresa.ID,
		NombreEmpresa:      empresa.NombreEmpresa,
		RFC:                empresa.RFC,
		PlanActual:         empresa.PlanActual,
		EstadoSuscripcion:  empresa.EstadoSuscripcion,
		MaxConductores:     empresa.MaxConductores,
		ConductoresActuales: totalConductores,
		ConductoresExtra:   empresa.ConductoresExtra,
		PeriodoInicio:      empresa.CurrentPeriodStart,
		PeriodoFin:         empresa.CurrentPeriodEnd,
		CreatedAt:          empresa.CreatedAt,
	}, nil
}

// ─── Cambiar plan ──────────────────────────────────────────────
func (s *Service) CambiarPlan(adminID string, req CambiarPlanRequest) error {
    empresa, err := s.repo.GetEmpresaByAdminID(adminID)
    if err != nil {
        return fmt.Errorf("empresa no encontrada")
    }

    if empresa.EstadoSuscripcion != EstadoActivo {
        return fmt.Errorf("la suscripción no está activa")
    }

    // ✅ Validar que haya cambios reales
    sinCambioPlan := req.PlanNuevo == empresa.PlanActual
    sinCambioExtra := req.ConductoresExtra == empresa.ConductoresExtra
    
    if sinCambioPlan && sinCambioExtra {
        return fmt.Errorf("no hay cambios en el plan")
    }

    totalConductores, err := s.repo.GetTotalConductoresByEmpresa(empresa.ID)
    if err != nil {
        return fmt.Errorf("error contando conductores: %w", err)
    }
	log.Print("Total conductores:",totalConductores)

    limiteNuevoPlan := LimitesConductores[req.PlanNuevo]

	log.Print("limite nuevo plan:",limiteNuevoPlan)

    // ✅ Downgrade: Profesional → Básico
    if req.PlanNuevo == PlanBasico && empresa.PlanActual == PlanProfesional {
		log.Print("downgrade")
        conductoresSobrantes := 0
        if totalConductores > limiteNuevoPlan {
            conductoresSobrantes = totalConductores - limiteNuevoPlan
        }

		log.Print("conductores sobrantes:",conductoresSobrantes)

        empresa.PlanActual = PlanBasico
        empresa.MaxConductores = limiteNuevoPlan
        empresa.ConductoresExtra = conductoresSobrantes

        if err := s.repo.UpdateEmpresa(empresa); err != nil {
            return fmt.Errorf("error actualizando empresa: %w", err)
        }

        if empresa.StripeSubscriptionID != "" && s.stripeCfg != nil {
            s.actualizarSuscripcionStripe(empresa, conductoresSobrantes)
        }
		log.Print("evaluar")

		if conductoresSobrantes > 0 {
			log.Print("generar factura")
            ahora := time.Now()
            yearEnd := ahora.AddDate(1, 0, 0)
            cargoExtra := float64(conductoresSobrantes) * PrecioConductorExtra
            
            factura := &Factura{
                EmpresaID:             empresa.ID,
                Subtotal:              cargoExtra,
                IVA:                   cargoExtra * 0.16,
                Total:                 cargoExtra * 1.16,
                Plan:                  PlanBasico,
                ConductoresBase:       limiteNuevoPlan,
                ConductoresExtra:      conductoresSobrantes,
                CargoConductoresExtra: cargoExtra,
                PeriodoInicio:         &ahora,
                PeriodoFin:            &yearEnd,
                Estado:                string(FacturaPendiente),
            }
            if err := s.repo.CrearFactura(factura); err != nil {
                log.Printf("Error creando factura por conductores extra: %v", err)
            }
			
        }
        _ = s.repo.RegistrarHistorial(empresa.ID, CambioCambioPlan,
            fmt.Sprintf("Downgrade: Profesional → Básico. %d conductores extra", conductoresSobrantes),
            map[string]interface{}{
                "plan_anterior":     "profesional",
                "plan_nuevo":        "basico",
                "conductores_extra": conductoresSobrantes,
            })

        return nil
    }

    // ✅ Upgrade: Básico → Profesional
    if req.PlanNuevo == PlanProfesional && empresa.PlanActual == PlanBasico {
        // Si los extra caben en el nuevo plan, eliminarlos
        if empresa.ConductoresExtra > 0 && totalConductores <= limiteNuevoPlan {
            empresa.ConductoresExtra = 0
        }

        empresa.PlanActual = PlanProfesional
        empresa.MaxConductores = limiteNuevoPlan

        if err := s.repo.UpdateEmpresa(empresa); err != nil {
            return fmt.Errorf("error actualizando empresa: %w", err)
        }

        if empresa.StripeSubscriptionID != "" && s.stripeCfg != nil {
            s.actualizarSuscripcionStripe(empresa, 0)
        }

        _ = s.repo.RegistrarHistorial(empresa.ID, CambioCambioPlan,
            "Upgrade: Básico → Profesional",
            map[string]interface{}{
                "plan_anterior": "basico",
                "plan_nuevo":    "profesional",
            })

        return nil
    }

    // ✅ Mismo plan, solo cambian conductores extra
    if req.PlanNuevo == empresa.PlanActual {
        empresa.ConductoresExtra = req.ConductoresExtra
        if err := s.repo.UpdateEmpresa(empresa); err != nil {
            return fmt.Errorf("error actualizando conductores extra: %w", err)
        }

        _ = s.repo.RegistrarHistorial(empresa.ID, CambioAgregarConductor,
            fmt.Sprintf("Conductores extra actualizados: %d", req.ConductoresExtra),
            nil)

        return nil
    }

    return fmt.Errorf("cambio de plan no válido")
}

func (s *Service) actualizarSuscripcionStripe(empresa *Empresa, conductoresExtra int) error {
    if s.stripeCfg == nil || s.stripeCfg.SecretKey == "" {
        log.Println("[STRIPE] No configurado, omitiendo actualización")
        return nil
    }
    if empresa.StripeSubscriptionID == "" {
        log.Println("[STRIPE] Sin subscription ID, omitiendo")
        return nil
    }
    
    // Si es un ID de prueba manual, no intentar actualizar en Stripe
    if strings.HasPrefix(empresa.StripeSubscriptionID, "sub_manual") ||
       strings.HasPrefix(empresa.StripeSubscriptionID, "sub_test_manual") {
        log.Printf("[STRIPE] Subscription manual '%s' - omitiendo actualización en Stripe", empresa.StripeSubscriptionID)
        return nil
    }

    stripe.Key = s.stripeCfg.SecretKey

    params := &stripe.SubscriptionItemListParams{
        Subscription: stripe.String(empresa.StripeSubscriptionID),
    }
    
    iter := subscriptionitem.List(params)
    for iter.Next() {
        item := iter.SubscriptionItem()
        
        newPriceID := s.stripeCfg.PriceBasico
        if empresa.PlanActual == PlanProfesional {
            newPriceID = s.stripeCfg.PricePro
        }
        
        _, err := subscriptionitem.Update(item.ID, &stripe.SubscriptionItemParams{
            Price:              stripe.String(newPriceID),
            ProrationBehavior:  stripe.String("always_invoice"),
        })
        if err != nil {
            log.Printf("[STRIPE] Error actualizando (no crítico): %v", err)
            // No retornar error - el cambio en BD ya se hizo
            return nil
        }
    }

    if err := iter.Err(); err != nil {
        log.Printf("[STRIPE] Error iterando (no crítico): %v", err)
        return nil
    }

    return nil
}

// ─── Agregar conductores extra ─────────────────────────────────

func (s *Service) AgregarConductores(adminID string, cantidad int) error {
	if cantidad <= 0 {
		return fmt.Errorf("la cantidad debe ser mayor a 0")
	}

	empresa, err := s.repo.GetEmpresaByAdminID(adminID)
	if err != nil {
		return fmt.Errorf("empresa no encontrada")
	}

	if empresa.EstadoSuscripcion != EstadoActivo {
		return fmt.Errorf("la suscripción no está activa")
	}

	nuevoExtra := empresa.ConductoresExtra + cantidad
	if err := s.repo.ActualizarConductoresExtra(empresa.ID, nuevoExtra); err != nil {
		return fmt.Errorf("error actualizando conductores extra: %w", err)
	}

	_ = s.repo.RegistrarHistorial(empresa.ID, CambioAgregarConductor,
		fmt.Sprintf("Se agregaron %d conductores extra (total: %d)", cantidad, nuevoExtra),
		map[string]interface{}{
			"cantidad":     cantidad,
			"total_extra":  nuevoExtra,
			"cargo_anual":  float64(cantidad) * PrecioConductorExtra,
		})

	return nil
}

// ─── Cancelar suscripción ──────────────────────────────────────

// billing/service.go
func (s *Service) CancelarSuscripcion(adminID string) error {
    empresa, err := s.repo.GetEmpresaByAdminID(adminID)
    if err != nil {
        return fmt.Errorf("empresa no encontrada")
    }

    // Cancelar en Stripe primero
    if empresa.StripeSubscriptionID != "" && s.stripeCfg != nil && s.stripeCfg.SecretKey != "" {
        stripe.Key = s.stripeCfg.SecretKey
        _, err := subscription.Cancel(empresa.StripeSubscriptionID, &stripe.SubscriptionCancelParams{})
        if err != nil {
            log.Printf("Error cancelando suscripción en Stripe: %v", err)
            // Continuar con la cancelación local aunque falle Stripe
        }
    }

    if err := s.repo.ActualizarEstadoSuscripcion(empresa.ID, EstadoCancelado); err != nil {
        return fmt.Errorf("error cancelando suscripción: %w", err)
    }

    _ = s.repo.RegistrarHistorial(empresa.ID, CambioCancelacion,
        "Suscripción cancelada por el administrador", nil)

    return nil
}


func (s *Service) IsSuscripcionActiva(adminID string) (bool, error) {
    empresa, err := s.repo.GetEmpresaByAdminID(adminID)
    if err != nil {
        return false, nil // Sin empresa = no activa
    }
    return empresa.EstadoSuscripcion == EstadoActivo, nil
}

// ─── Facturas ──────────────────────────────────────────────────

func (s *Service) ListFacturas(adminID string, page, limit int) ([]*Factura, int, error) {
	empresa, err := s.repo.GetEmpresaByAdminID(adminID)
	if err != nil {
		return nil, 0, fmt.Errorf("empresa no encontrada")
	}

	offset := (page - 1) * limit
	facturas, err := s.repo.ListFacturasByEmpresa(empresa.ID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.CountFacturasByEmpresa(empresa.ID)
	if err != nil {
		total = len(facturas)
	}

	return facturas, total, nil
}

// ─── Historial ─────────────────────────────────────────────────

func (s *Service) ListHistorial(adminID string, limit int) ([]*HistorialSuscripcion, error) {
	empresa, err := s.repo.GetEmpresaByAdminID(adminID)
	if err != nil {
		return nil, fmt.Errorf("empresa no encontrada")
	}

	return s.repo.ListHistorial(empresa.ID, limit)
}

// ─── Webhook Stripe ────────────────────────────────────────────

func (s *Service) ProcesarWebhook(payload []byte, signature string) error {
	if s.stripeCfg == nil || s.stripeCfg.WebhookSecret == "" {
		return fmt.Errorf("webhook secret no configurado")
	}

	 event, err := webhook.ConstructEventWithOptions(
        payload,
        signature,
        s.stripeCfg.WebhookSecret,
        webhook.ConstructEventOptions{
            IgnoreAPIVersionMismatch: true,  // ← AGREGAR ESTO
        },
    )

	if err != nil {
        log.Printf("[WEBHOOK] Error verificando firma: %v", err)
        return fmt.Errorf("error verificando webhook: %w", err)
    }

	log.Printf("[WEBHOOK] Evento recibido: %s", event.Type)

	switch event.Type {
	case "checkout.session.completed":
		return s.handleCheckoutCompletado(event)

	case "invoice.paid":
		return s.handleInvoicePagado(event)

	case "customer.subscription.deleted":
		return s.handleSuscripcionCancelada(event)

	case "invoice.payment_failed":
		return s.handlePagoFallido(event)

	case "customer.subscription.updated":
    return s.handleSuscripcionActualizada(event)

	default:
		log.Printf("Webhook no manejado: %s", event.Type)
	}

	return nil
}



func (s *Service) handleSuscripcionActualizada(event stripe.Event) error {
    var sub stripe.Subscription
    if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
        return fmt.Errorf("error decodificando subscription: %w", err)
    }

    empresa, err := s.repo.GetEmpresaByStripeSubscriptionID(sub.ID)
    if err != nil {
        return fmt.Errorf("empresa no encontrada para subscription %s", sub.ID)
    }

    // Actualizar período
    periodStart := time.Unix(sub.CurrentPeriodStart, 0)
    periodEnd := time.Unix(sub.CurrentPeriodEnd, 0)
    _ = s.repo.UpdateSuscripcionStripe(empresa.ID, sub.ID, periodStart, periodEnd)

    // Actualizar estado si cambió
    estado := EstadoActivo
    switch sub.Status {
    case stripe.SubscriptionStatusActive:
        estado = EstadoActivo
    case stripe.SubscriptionStatusPastDue:
        estado = EstadoPendiente
    case stripe.SubscriptionStatusCanceled:
        estado = EstadoCancelado
    case stripe.SubscriptionStatusUnpaid:
        estado = EstadoExpirado
    }
    
    if empresa.EstadoSuscripcion != estado {
        _ = s.repo.ActualizarEstadoSuscripcion(empresa.ID, estado)
        _ = s.repo.RegistrarHistorial(empresa.ID, CambioRenovacion,
            fmt.Sprintf("Estado de suscripción actualizado por Stripe: %s → %s", 
                empresa.EstadoSuscripcion, estado), nil)
    }

    return nil
}

// billing/service.go
func (s *Service) handleCheckoutCompletado(event stripe.Event) error {
    var sess stripe.CheckoutSession
    if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
        return fmt.Errorf("error decodificando checkout.session: %w", err)
    }

    empresaID := sess.Metadata["empresa_id"]
    if empresaID == "" {
        return fmt.Errorf("empresa_id no encontrado en metadata")
    }

	plan := Plan(sess.Metadata["plan"])
    if plan == "" {
        plan = PlanBasico // Default
    }

    // Activar suscripción
    if err := s.repo.ActivarSuscripcionConPlan(empresaID, sess.Subscription.ID, plan); err != nil {
        return fmt.Errorf("error activando suscripción: %w", err)
    }

    // Actualizar período si viene en la sesión
    if sess.Subscription != nil {
        now := time.Now()
        yearEnd := now.AddDate(1, 0, 0)
        _ = s.repo.UpdateSuscripcionStripe(empresaID, sess.Subscription.ID, now, yearEnd)
    }

    // Actualizar pago con nil-check
    paymentIntentID := ""
    if sess.PaymentIntent != nil {
        paymentIntentID = sess.PaymentIntent.ID
    }
    
    pagos, _ := s.repo.GetPagosByFactura(sess.Metadata["factura_id"])
    for _, p := range pagos {
        if p.ProviderCheckoutID == sess.ID {
            _ = s.repo.ActualizarEstadoPago(p.ID, PagoCompletado, paymentIntentID)
            break
        }
    }

    _ = s.repo.RegistrarHistorial(empresaID, CambioActivacion,
        "Suscripción activada después de pago exitoso", map[string]interface{}{
            "stripe_session_id": sess.ID,
        })

    return nil
}

// billing/service.go
func (s *Service) handleInvoicePagado(event stripe.Event) error {
    var inv stripe.Invoice
    if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
        return fmt.Errorf("error decodificando invoice: %w", err)
    }

    factura, err := s.repo.GetFacturaByStripeInvoiceID(inv.ID)
    if err != nil {
        // Crear nueva factura para renovación
        empresa, err := s.repo.GetEmpresaByStripeCustomerID(inv.Customer.ID)
        if err != nil {
            return fmt.Errorf("empresa no encontrada para customer %s", inv.Customer.ID)
        }

        metodoPago := ""
        if inv.PaymentIntent != nil && inv.PaymentIntent.PaymentMethod != nil {
            metodoPago = string(inv.PaymentIntent.PaymentMethod.Type)
        }

        subtotal := float64(inv.Subtotal) / 100
        iva := float64(inv.Tax) / 100 // ✅ Usar el tax de Stripe
        if iva == 0 {
            iva = subtotal * 0.16 // Fallback: calcular 16% si Stripe no reporta tax
        }

        ahora := time.Now()
        yearEnd := ahora.AddDate(1, 0, 0)

        factura = &Factura{
            EmpresaID:             empresa.ID,
            StripeInvoiceID:       inv.ID,
            StripePaymentIntentID: func() string {
                if inv.PaymentIntent != nil {
                    return inv.PaymentIntent.ID
                }
                return ""
            }(),
            Subtotal:              subtotal,
            IVA:                   iva,
            Total:                 float64(inv.Total) / 100,
            Plan:                  empresa.PlanActual,
            ConductoresBase:       empresa.MaxConductores,
            ConductoresExtra:      empresa.ConductoresExtra,
            CargoConductoresExtra: float64(empresa.ConductoresExtra) * PrecioConductorExtra,
            PeriodoInicio:         &ahora,
            PeriodoFin:            &yearEnd,
            Estado:                string(FacturaPagado),
            MetodoPago:            metodoPago,
        }
        if err := s.repo.CrearFactura(factura); err != nil {
            log.Printf("Error creando factura desde webhook: %v", err)
        }
        
        // Actualizar período de suscripción
        _ = s.repo.UpdateSuscripcionStripe(empresa.ID, empresa.StripeSubscriptionID, ahora, yearEnd)
        _ = s.repo.RegistrarHistorial(empresa.ID, CambioRenovacion,
            fmt.Sprintf("Renovación automática: $%.2f MXN", factura.Total), nil)
    } else {
        now := time.Now()
        metodoPago := ""
        if inv.PaymentIntent != nil && inv.PaymentIntent.PaymentMethod != nil {
            metodoPago = string(inv.PaymentIntent.PaymentMethod.Type)
        }
        _ = s.repo.ActualizarEstadoFactura(factura.ID, FacturaPagado, &now, metodoPago)
    }

    return nil
}
func (s *Service) handleSuscripcionCancelada(event stripe.Event) error {
    var sub stripe.Subscription
    if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
        return fmt.Errorf("error decodificando subscription: %w", err)
    }

    empresa, err := s.repo.GetEmpresaByStripeSubscriptionID(sub.ID)
    if err != nil {
        return fmt.Errorf("empresa no encontrada para subscription %s", sub.ID)
    }

    _ = s.repo.ActualizarEstadoSuscripcion(empresa.ID, EstadoCancelado)
    _ = s.repo.RegistrarHistorial(empresa.ID, CambioCancelacion,
        "Suscripción cancelada desde Stripe", nil)

    return nil
}

func (s *Service) handlePagoFallido(event stripe.Event) error {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return fmt.Errorf("error decodificando invoice: %w", err)
	}

	factura, err := s.repo.GetFacturaByStripeInvoiceID(inv.ID)
	if err == nil {
		_ = s.repo.ActualizarEstadoFactura(factura.ID, FacturaVencido, nil, "")
		_ = s.repo.RegistrarHistorial(factura.EmpresaID, CambioPagoFallido,
			"Pago de factura fallido", nil)
	}

	return nil
}

// ─── Calcular precio ───────────────────────────────────────────

func (s *Service) CalcularPrecio(plan Plan, conductoresExtra int) CalcularPrecioResponse {
	subtotal, iva, total := CalcularPrecioTotal(plan, conductoresExtra)
	cargoExtra := float64(conductoresExtra) * PrecioConductorExtra

	return CalcularPrecioResponse{
		Plan:                plan,
		ConductoresBase:     LimitesConductores[plan],
		ConductoresExtra:    conductoresExtra,
		CargoExtra:          cargoExtra,
		Subtotal:            subtotal,
		IVA:                 iva,
		Total:               total,
		PrecioConductorExtra: PrecioConductorExtra,
	}
}

// ─── Admin: Listar todas las empresas ──────────────────────────

func (s *Service) ListAllEmpresas() ([]*Empresa, error) {
	return s.repo.ListEmpresas()
}