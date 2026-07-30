package billing

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"saferoute/internal/security"
)

// Client es el cliente HTTP para comunicarse con el Billing Service independiente
type Client struct {
	billingServiceURL string
	internalAPIKey    string
	servicePrivateKey ed25519.PrivateKey
	httpClient        *http.Client
}

func NewClient(billingServiceURL, internalAPIKey string, servicePrivateKey ed25519.PrivateKey) *Client {
	return &Client{
		billingServiceURL: billingServiceURL,
		internalAPIKey:    internalAPIKey,
		servicePrivateKey: servicePrivateKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) sendSignedRequest(method, urlPath string, body interface{}) (*http.Response, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("error serializando cuerpo: %w", err)
		}
	}

	req, err := http.NewRequest(method, c.billingServiceURL+urlPath, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	// Intentar firmar la petición si la clave privada está configurada
	if c.servicePrivateKey != nil {
		timestamp := time.Now().Unix()
		sig, err := security.SignRequest(c.servicePrivateKey, method, urlPath, timestamp, bodyBytes)
		if err == nil {
			req.Header.Set("X-Signature", sig)
			req.Header.Set("X-Key-ID", "saferoute-api")
			req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))
		} else {
			log.Printf("[BILLING-CLIENT] Advertencia: falló la firma de la petición: %v", err)
		}
	}

	// Agregar la API Key interna como fallback/adicional
	req.Header.Set("X-Internal-API-Key", c.internalAPIKey)

	return c.httpClient.Do(req)
}

// CrearEmpresa llama al billing service para crear una empresa
func (c *Client) CrearEmpresa(adminID string, req CrearEmpresaRequest) (*CheckoutResponse, error) {
	httpReq, err := http.NewRequest("POST", c.billingServiceURL+"/api/internal/billing/empresa/crear", nil)
	if err != nil {
		return nil, err
	}

	bodyBytes, _ := json.Marshal(req)
	httpReq.Body = ioNopCloser(bytes.NewBuffer(bodyBytes))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Admin-ID", adminID)
	httpReq.Header.Set("X-Internal-API-Key", c.internalAPIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error conectando al billing service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if msg, ok := errResp["error"].(string); ok {
			return nil, fmt.Errorf(msg)
		}
		return nil, fmt.Errorf("error en billing service: código %d", resp.StatusCode)
	}

	var result CheckoutResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decodificando respuesta: %w", err)
	}

	return &result, nil
}

// GetMiEmpresa obtiene la empresa del admin desde el billing service
func (c *Client) GetMiEmpresa(adminID string) (*EmpresaResponse, error) {
	req, err := http.NewRequest("GET", c.billingServiceURL+"/api/internal/billing/empresa", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Admin-ID", adminID)
	req.Header.Set("X-Internal-API-Key", c.internalAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error conectando al billing service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if msg, ok := errResp["error"].(string); ok {
			return nil, fmt.Errorf(msg)
		}
		return nil, fmt.Errorf("empresa no encontrada: código %d", resp.StatusCode)
	}

	var result EmpresaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decodificando respuesta: %w", err)
	}

	return &result, nil
}

// IsSuscripcionActiva verifica si la suscripción del admin está activa
func (c *Client) IsSuscripcionActiva(adminID string) (bool, error) {
	req, err := http.NewRequest("GET", c.billingServiceURL+"/api/internal/billing/check-activa", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-Admin-ID", adminID)
	req.Header.Set("X-Internal-API-Key", c.internalAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, nil // Si no hay conexión, asumir no activa
	}
	defer resp.Body.Close()

	var result struct {
		Activa bool `json:"activa"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, nil
	}

	return result.Activa, nil
}

// GetTotalConductoresByEmpresa obtiene el total de conductores de una empresa
func (c *Client) GetTotalConductoresByEmpresa(empresaID string) (int, error) {
	// Este endpoint se consulta directamente desde la BD compartida
	// o se puede agregar al billing service
	return 0, nil
}

// GetEmpresaByAdminID obtiene la empresa por admin ID
func (c *Client) GetEmpresaByAdminID(adminID string) (*Empresa, error) {
	resp, err := c.sendSignedRequest("GET", "/api/internal/billing/empresa", nil)
	if err != nil {
		return nil, fmt.Errorf("error conectando al billing service: %w", err)
	}
	defer resp.Body.Close()

	// Necesitamos pasar el adminID como header
	// Como sendSignedRequest no soporta headers personalizados, usamos el método directo
	req, _ := http.NewRequest("GET", c.billingServiceURL+"/api/internal/billing/empresa", nil)
	req.Header.Set("X-Admin-ID", adminID)
	req.Header.Set("X-Internal-API-Key", c.internalAPIKey)

	resp2, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error conectando al billing service: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("empresa no encontrada")
	}

	var empresa Empresa
	if err := json.NewDecoder(resp2.Body).Decode(&empresa); err != nil {
		return nil, fmt.Errorf("error decodificando: %w", err)
	}

	return &empresa, nil
}

// CambiarPlan cambia el plan de la empresa
func (c *Client) CambiarPlan(adminID string, req CambiarPlanRequest) error {
	bodyBytes, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("PUT", c.billingServiceURL+"/api/internal/billing/empresa/cambiar-plan", bytes.NewBuffer(bodyBytes))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Admin-ID", adminID)
	httpReq.Header.Set("X-Internal-API-Key", c.internalAPIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("error conectando al billing service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if msg, ok := errResp["error"].(string); ok {
			return fmt.Errorf(msg)
		}
		return fmt.Errorf("error en billing service: código %d", resp.StatusCode)
	}

	return nil
}

// AgregarConductores agrega conductores extra
func (c *Client) AgregarConductores(adminID string, cantidad int) error {
	body := map[string]int{"cantidad": cantidad}
	bodyBytes, _ := json.Marshal(body)
	httpReq, _ := http.NewRequest("POST", c.billingServiceURL+"/api/internal/billing/empresa/conductores", bytes.NewBuffer(bodyBytes))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Admin-ID", adminID)
	httpReq.Header.Set("X-Internal-API-Key", c.internalAPIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("error conectando al billing service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if msg, ok := errResp["error"].(string); ok {
			return fmt.Errorf(msg)
		}
		return fmt.Errorf("error en billing service: código %d", resp.StatusCode)
	}

	return nil
}

// QuitarConductores quita conductores extra
func (c *Client) QuitarConductores(adminID string, cantidad int) error {
	body := map[string]int{"cantidad": cantidad}
	bodyBytes, _ := json.Marshal(body)
	httpReq, _ := http.NewRequest("POST", c.billingServiceURL+"/api/internal/billing/empresa/conductores/quitar", bytes.NewBuffer(bodyBytes))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Admin-ID", adminID)
	httpReq.Header.Set("X-Internal-API-Key", c.internalAPIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("error conectando al billing service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if msg, ok := errResp["error"].(string); ok {
			return fmt.Errorf(msg)
		}
		return fmt.Errorf("error en billing service: código %d", resp.StatusCode)
	}

	return nil
}

// CancelarSuscripcion cancela la suscripción
func (c *Client) CancelarSuscripcion(adminID string) error {
	httpReq, _ := http.NewRequest("POST", c.billingServiceURL+"/api/internal/billing/empresa/cancelar", nil)
	httpReq.Header.Set("X-Admin-ID", adminID)
	httpReq.Header.Set("X-Internal-API-Key", c.internalAPIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("error conectando al billing service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if msg, ok := errResp["error"].(string); ok {
			return fmt.Errorf(msg)
		}
		return fmt.Errorf("error en billing service: código %d", resp.StatusCode)
	}

	return nil
}

// ListFacturas lista las facturas
func (c *Client) ListFacturas(adminID string, page, limit int) ([]*Factura, int, error) {
	url := fmt.Sprintf("%s/api/internal/billing/facturas?page=%d&limit=%d", c.billingServiceURL, page, limit)
	httpReq, _ := http.NewRequest("GET", url, nil)
	httpReq.Header.Set("X-Admin-ID", adminID)
	httpReq.Header.Set("X-Internal-API-Key", c.internalAPIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("error conectando al billing service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("error obteniendo facturas: código %d", resp.StatusCode)
	}

	var result struct {
		Facturas []*Factura `json:"facturas"`
		Total    int        `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("error decodificando: %w", err)
	}

	return result.Facturas, result.Total, nil
}

// ListHistorial lista el historial de la suscripción
func (c *Client) ListHistorial(adminID string, limit int) ([]*HistorialSuscripcion, error) {
	url := fmt.Sprintf("%s/api/internal/billing/historial?limit=%d", c.billingServiceURL, limit)
	httpReq, _ := http.NewRequest("GET", url, nil)
	httpReq.Header.Set("X-Admin-ID", adminID)
	httpReq.Header.Set("X-Internal-API-Key", c.internalAPIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error conectando al billing service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error obteniendo historial: código %d", resp.StatusCode)
	}

	var historial []*HistorialSuscripcion
	if err := json.NewDecoder(resp.Body).Decode(&historial); err != nil {
		return nil, fmt.Errorf("error decodificando: %w", err)
	}

	return historial, nil
}

// CalcularPrecio calcula el precio de un plan (se hace localmente)
func (c *Client) CalcularPrecio(plan Plan, conductoresExtra int) CalcularPrecioResponse {
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

// ProcesarWebhook envía el webhook al billing service
func (c *Client) ProcesarWebhook(payload []byte, signature string) error {
	req, err := http.NewRequest("POST", c.billingServiceURL+"/api/webhooks/stripe", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", signature)
	req.Header.Set("X-Internal-API-Key", c.internalAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error conectando al billing service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error en webhook: código %d", resp.StatusCode)
	}

	return nil
}

// ListAllEmpresas lista todas las empresas (admin)
func (c *Client) ListAllEmpresas() ([]*Empresa, error) {
	resp, err := c.sendSignedRequest("GET", "/api/internal/billing/empresas", nil)
	if err != nil {
		return nil, fmt.Errorf("error conectando al billing service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error listando empresas: código %d", resp.StatusCode)
	}

	var empresas []*Empresa
	if err := json.NewDecoder(resp.Body).Decode(&empresas); err != nil {
		return nil, fmt.Errorf("error decodificando: %w", err)
	}

	return empresas, nil
}

// ioNopCloser es un helper para crear un ReadCloser desde un Reader
func ioNopCloser(r *bytes.Buffer) ioReadCloser {
	return &nopCloser{r}
}

type ioReadCloser interface {
	Read(p []byte) (n int, err error)
	Close() error
}

type nopCloser struct {
	*bytes.Buffer
}

func (nopCloser) Close() error { return nil }