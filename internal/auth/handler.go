package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"saferoute/internal/billing"
	"saferoute/internal/common"
	"saferoute/internal/middleware"
)

type Handler struct {
	authSvc   *AuthService
	jwtSecret string
}

func NewHandler(authSvc *AuthService, jwtSecret string) *Handler {
	return &Handler{
		authSvc:   authSvc,
		jwtSecret: jwtSecret,
	}
}

func (h *Handler) LoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		if err := ValidateLogin(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		result, err := h.authSvc.Login(req)
		if err != nil {
			common.WriteError(w, http.StatusUnauthorized, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func (h *Handler) RegisterHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req RegisterRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		if err := ValidateRegister(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		log.Printf("[REGISTER] Datos recibidos - Email: %s, Nombre: %s, Teléfono: '%s', Tipo: %s",
			req.Email, req.Nombre, req.Telefono, req.Tipo)

		result, err := h.authSvc.Register(req)
		if err != nil {
			log.Printf("[REGISTER] Error: %v", err)
			common.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		log.Printf("[REGISTER] Usuario creado: %s, Teléfono guardado: '%s'",
			result.Email, req.Telefono)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(result)
	}
}

func (h *Handler) ValidateTokenHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos inválidos")
			return
		}

		if req.Token == "" {
			common.WriteError(w, http.StatusBadRequest, "token es requerido")
			return
		}

		result, err := h.authSvc.ValidateToken(req.Token)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(ValidateTokenResponse{
				Valid: false,
				Error: err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ValidateTokenResponse{
			Valid:  true,
			UserID: result["user_id"].(string),
			Email:  result["email"].(string),
			Tipo:   result["tipo"].(string),
			Nombre: func() string {
				if n, ok := result["nombre"].(string); ok {
					return n
				}
				return ""
			}(),
		})
	}
}

func (h *Handler) GetUserHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		userID := vars["id"]

		if userID == "" {
			common.WriteError(w, http.StatusBadRequest, "id de usuario requerido")
			return
		}

		user, err := h.authSvc.GetUserByID(userID)
		if err != nil {
			common.WriteError(w, http.StatusNotFound, "usuario no encontrado")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(UserProfileResponse{
			ID:       user.ID,
			Email:    user.Email,
			Nombre:   user.Nombre,
			Tipo:     user.Tipo,
			Telefono: user.Telefono,
		})
	}
}

// RegistrarAdminPublicoHandler - Endpoint PÚBLICO para registro de admin.
// Crea el admin pero con estado "pendiente_pago". Sin plan = sin acceso.
func (h *Handler) RegistrarAdminPublicoHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            Email    string `json:"email"`
            Password string `json:"password"`
            Nombre   string `json:"nombre"`
            Telefono string `json:"telefono"`
        }

        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            common.WriteError(w, http.StatusBadRequest, "datos inválidos")
            return
        }

        if req.Email == "" || req.Password == "" || req.Nombre == "" {
            common.WriteError(w, http.StatusBadRequest, "email, password y nombre son requeridos")
            return
        }

        if len(req.Password) < 6 {
            common.WriteError(w, http.StatusBadRequest, "la contraseña debe tener al menos 6 caracteres")
            return
        }

        // Llamar al Auth Service (él crea admin + empresa pendiente)
        result, err := h.authSvc.RegisterAdminPublico(req.Email, req.Password, req.Nombre, req.Telefono)
        if err != nil {
            common.WriteError(w, http.StatusConflict, err.Error())
            return
        }

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "id":      result.UserID,
            "token":   result.Token,
            "nombre":  result.Nombre,
            "email":   result.Email,
            "tipo":    result.Tipo,
            "status":  "pendiente_pago",
            "mensaje": "Cuenta creada. Elige un plan para acceder a todas las funcionalidades.",
            "redirect": "/precios",
        })
    }
}



func (h *Handler) RegistrarConductorHandler(billingSvc *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID := middleware.GetUserID(r)
        if adminID == "" {
            common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
            return
        }

		if billingSvc != nil {
            empresa, err := billingSvc.GetEmpresaByAdminID(adminID)
            if err != nil {
                common.WriteError(w, http.StatusForbidden, 
                    "no tienes una empresa registrada. Crea tu plan primero.")
                return
            }
            if empresa.EstadoSuscripcion != billing.EstadoActivo {
                common.WriteError(w, http.StatusForbidden, 
                    "tu suscripción no está activa")
                return
            }
            
            total, _ := billingSvc.GetTotalConductoresByEmpresa(empresa.ID)
            limite := empresa.MaxConductores + empresa.ConductoresExtra
            if total >= limite {
                common.WriteError(w, http.StatusForbidden,
                    fmt.Sprintf("límite de conductores alcanzado (%d/%d). Agrega más conductores extra o actualiza tu plan.",
                        total, limite))
                return
            }
        }

		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Nombre   string `json:"nombre"`
			Telefono string `json:"telefono"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos inválidos")
			return
		}

		validateReq := RegisterRequest{
			Email:    req.Email,
			Password: req.Password,
			Nombre:   req.Nombre,
			Tipo:     "conductor",
			Telefono: req.Telefono,
		}
		if err := ValidateRegister(&validateReq); err != nil {
			common.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		id, err := h.authSvc.RegisterConductor(validateReq.Email, validateReq.Password, validateReq.Nombre, validateReq.Telefono, adminID)
		if err != nil {
			common.WriteError(w, http.StatusConflict, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"id":     id,
			"status": "conductor registrado",
			"email":  req.Email,
		})
	}
}
