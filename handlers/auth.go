package handlers

import (
	"encoding/json"
	"net/http"

	"saferoute/models"
	"saferoute/pipes"
	"saferoute/services"
)

// LoginHandler es el handler HTTP delegador: decodifica → Pipe → Service → respuesta.
// Toda la lógica de negocio y acceso a BD vive en AuthService.
func LoginHandler(authSvc *services.AuthService, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Decodificar DTO de entrada
		var req models.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		// 2. Pipe: Validar el DTO
		if err := pipes.ValidateLogin(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// 3. Service: Ejecutar lógica de negocio
		response, err := authSvc.Login(req, jwtSecret)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}

		// 4. Responder con el DTO de salida
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// RegisterHandler es el handler HTTP delegador para registro de usuarios.
func RegisterHandler(authSvc *services.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Decodificar DTO de entrada
		var req models.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		// 2. Pipe: Validar y normalizar el DTO (modifica req in-place)
		if err := pipes.ValidateRegister(&req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// 3. Service: Ejecutar lógica de negocio
		id, err := authSvc.Register(req)
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}

		// 4. Responder con el DTO de salida
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"id":     id,
			"status": "creado",
		})
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