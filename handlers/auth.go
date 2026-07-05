package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"saferoute/models"
	"saferoute/pipes"
	"saferoute/services"
)

// LoginHandler es el handler HTTP delegador: decodifica → Pipe → Service → respuesta.
func LoginHandler(authSvc *services.AuthService, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		// 2. Pipe: Validar y normalizar el DTO de login
		if err := pipes.ValidateLogin(&req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// 3. Service: Validar credenciales y generar JWT
		result, err := authSvc.Login(req)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func RegisterHandler(authSvc *services.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.RegisterRequest

		// Asegúrate de decodificar correctamente el body
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		// 2. Pipe: Validar y sanitizar el DTO de registro
		if err := pipes.ValidateRegister(&req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		log.Printf("📝 [REGISTER] Datos recibidos - Email: %s, Nombre: %s, Teléfono: '%s', Tipo: %s",
			req.Email, req.Nombre, req.Telefono, req.Tipo)

		// Validar que el teléfono no venga vacío si es requerido
		if req.Telefono == "" {
			log.Printf("⚠️ [REGISTER] Teléfono vacío para usuario %s", req.Email)
		}

		result, err := authSvc.Register(req)
		if err != nil {
			log.Printf("❌ [REGISTER] Error: %v", err)
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		log.Printf("✅ [REGISTER] Usuario creado: %s, Teléfono guardado: '%s'",
			result.Email, req.Telefono)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(result)
	}
}

func writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Error: message,
		Code:  code,
	})
}
