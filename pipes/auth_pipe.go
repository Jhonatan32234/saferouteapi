package pipes

import (
	"fmt"
	"strings"

	"saferoute/models"
)

// ValidateLogin valida que el DTO de login tenga los campos requeridos.
// Devuelve un error descriptivo si la validación falla.
func ValidateLogin(req *models.LoginRequest) error {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" {
		return fmt.Errorf("el campo 'email' es requerido")
	}
	if !strings.Contains(req.Email, "@") {
		return fmt.Errorf("el campo 'email' no tiene un formato válido")
	}
	if strings.TrimSpace(req.Password) == "" {
		return fmt.Errorf("el campo 'password' es requerido")
	}
	return nil
}

// ValidateRegister valida el DTO de registro con todas las reglas de negocio.
// Normaliza el email a minúsculas como efecto secundario.
func ValidateRegister(req *models.RegisterRequest) error {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Nombre = strings.TrimSpace(req.Nombre)
	req.Telefono = strings.TrimSpace(req.Telefono)

	if req.Email == "" {
		return fmt.Errorf("el campo 'email' es requerido")
	}
	if !strings.Contains(req.Email, "@") {
		return fmt.Errorf("el campo 'email' no tiene un formato válido")
	}
	if len(req.Password) < 6 {
		return fmt.Errorf("la contraseña debe tener al menos 6 caracteres")
	}
	if req.Nombre == "" {
		return fmt.Errorf("el campo 'nombre' es requerido")
	}
	if req.Tipo == "" {
		req.Tipo = "conductor"
	}
	if req.Tipo != "conductor" {
		return fmt.Errorf("el campo 'tipo' solo puede ser 'conductor' para el registro público")
	}
	return nil
}
