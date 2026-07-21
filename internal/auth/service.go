package auth

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
	"saferoute/internal/user"
)

type AuthService struct {
	authServiceURL    string
	internalAPIKey    string
	servicePrivateKey ed25519.PrivateKey
	httpClient        *http.Client
}

func NewAuthService(authServiceURL, internalAPIKey string, servicePrivateKey ed25519.PrivateKey) *AuthService {
	return &AuthService{
		authServiceURL:    authServiceURL,
		internalAPIKey:    internalAPIKey,
		servicePrivateKey: servicePrivateKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *AuthService) sendSignedRequest(method, urlPath string, body interface{}) (*http.Response, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("error serializando cuerpo: %w", err)
		}
	}

	req, err := http.NewRequest(method, s.authServiceURL+urlPath, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	// Intentar firmar la petición si la clave privada está configurada
	if s.servicePrivateKey != nil {
		timestamp := time.Now().Unix()
		sig, err := security.SignRequest(s.servicePrivateKey, method, urlPath, timestamp, bodyBytes)
		if err == nil {
			req.Header.Set("X-Signature", sig)
			req.Header.Set("X-Key-ID", "saferoute-api")
			req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))
		} else {
			log.Printf("[AUTH-CLIENT] Advertencia: falló la firma de la petición: %v", err)
		}
	}

	// Agregar la API Key interna como fallback/adicional
	req.Header.Set("X-Internal-API-Key", s.internalAPIKey)

	return s.httpClient.Do(req)
}

func (s *AuthService) Login(req LoginRequest) (AuthResponse, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return AuthResponse{}, err
	}

	resp, err := s.httpClient.Post(s.authServiceURL+"/auth/login", "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return AuthResponse{}, fmt.Errorf("error conectando al servicio de autenticación: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if msg, ok := errResp["error"].(string); ok {
			return AuthResponse{}, fmt.Errorf(msg)
		}
		return AuthResponse{}, fmt.Errorf("error en servicio de autenticación: código %d", resp.StatusCode)
	}

	var result AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return AuthResponse{}, fmt.Errorf("error decodificando respuesta: %w", err)
	}

	return result, nil
}

func (s *AuthService) Register(req RegisterRequest) (AuthResponse, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return AuthResponse{}, err
	}

	resp, err := s.httpClient.Post(s.authServiceURL+"/auth/register", "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return AuthResponse{}, fmt.Errorf("error conectando al servicio de autenticación: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if msg, ok := errResp["error"].(string); ok {
			return AuthResponse{}, fmt.Errorf(msg)
		}
		return AuthResponse{}, fmt.Errorf("error en servicio de autenticación: código %d", resp.StatusCode)
	}

	var result AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return AuthResponse{}, fmt.Errorf("error decodificando respuesta: %w", err)
	}

	return result, nil
}

// RegisterAdminPublico registra un admin desde el endpoint público.
// El Auth Service crea el admin + empresa pendiente.
func (s *AuthService) RegisterAdminPublico(email, password, nombre, telefono string) (AuthResponse, error) {
    reqBody := map[string]string{
        "email":    email,
        "password": password,
        "nombre":   nombre,
        "telefono": telefono,
    }

    // Llamada firmada al Auth Service
    resp, err := s.sendSignedRequest("POST", "/auth/internal/registrar-admin-publico", reqBody)
    if err != nil {
        return AuthResponse{}, fmt.Errorf("error conectando al servicio de autenticación: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        var errResp map[string]interface{}
        json.NewDecoder(resp.Body).Decode(&errResp)
        if msg, ok := errResp["error"].(string); ok {
            return AuthResponse{}, fmt.Errorf(msg)
        }
        return AuthResponse{}, fmt.Errorf("error en servicio de autenticación: código %d", resp.StatusCode)
    }

    var result AuthResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return AuthResponse{}, fmt.Errorf("error decodificando respuesta: %w", err)
    }

    return result, nil
}

// saferoute/internal/auth/auth_service.go

func (s *AuthService) RegisterConductor(email, password, nombre, telefono, adminID string) (string, error) {
    reqBody := map[string]string{
        "email":    email,
        "password": password,
        "nombre":   nombre,
        "telefono": telefono,
        "admin_id": adminID,  // ← PASAR adminID
    }

    resp, err := s.sendSignedRequest("POST", "/auth/internal/registrar-conductor", reqBody)
    if err != nil {
        return "", fmt.Errorf("error conectando al servicio de autenticación: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
        var errResp map[string]interface{}
        json.NewDecoder(resp.Body).Decode(&errResp)
        if msg, ok := errResp["error"].(string); ok {
            return "", fmt.Errorf(msg)
        }
        return "", fmt.Errorf("error en servicio de autenticación: código %d", resp.StatusCode)
    }

    var result struct {
        ID string `json:"id"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", fmt.Errorf("error decodificando respuesta: %w", err)
    }

    return result.ID, nil
}
func (s *AuthService) ValidateToken(tokenString string) (map[string]interface{}, error) {
	reqBody := map[string]string{
		"token": tokenString,
	}

	resp, err := s.sendSignedRequest("POST", "/auth/internal/validate", reqBody)
	if err != nil {
		return nil, fmt.Errorf("error conectando al servicio de autenticación: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token inválido o error en servicio de autenticación: código %d", resp.StatusCode)
	}

	var result ValidateTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decodificando respuesta: %w", err)
	}

	if !result.Valid {
		return nil, fmt.Errorf(result.Error)
	}

	return map[string]interface{}{
		"user_id": result.UserID,
		"email":   result.Email,
		"tipo":    result.Tipo,
		"nombre":  result.Nombre,
	}, nil
}

func (s *AuthService) GetUserByID(id string) (*user.UsuarioEntity, error) {
	resp, err := s.sendSignedRequest("GET", "/auth/internal/user/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("error conectando al servicio de autenticación: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("usuario no encontrado: código %d", resp.StatusCode)
	}

	var result UserProfileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decodificando respuesta: %w", err)
	}

	return &user.UsuarioEntity{
		ID:       result.ID,
		Email:    result.Email,
		Nombre:   result.Nombre,
		Tipo:     result.Tipo,
		Telefono: result.Telefono,
	}, nil
}
