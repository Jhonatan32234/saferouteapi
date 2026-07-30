package user

import (
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"

	"saferoute/internal/security"
)

type UsuarioEntity struct {
	ID           string
	Email        string
	EmailHash    string // ← NUEVO: hash para búsquedas
	PasswordHash string
	Nombre       string
	Tipo         string
	Telefono     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	UltimoAcceso *time.Time
}

type UsuarioPerfilConEstadisticas struct {
    UsuarioEntity
    ReportesCreados     int
    ReportesConfirmados int
}

func (u *UsuarioPerfilConEstadisticas) AfterLoad(key []byte) error {
    return u.UsuarioEntity.AfterLoad(key)
}

func (u *UsuarioEntity) BeforeSave(key []byte) error {
	var err error

	// Generar hash del email ANTES de encriptarlo
	if u.Email != "" {
		u.EmailHash = security.HashEmail(strings.ToLower(strings.TrimSpace(u.Email)), key)

		u.Email, err = security.Encrypt(u.Email, key)
		if err != nil {
			return fmt.Errorf("error encriptando email: %w", err)
		}
	}

	// Encriptar nombre
	if u.Nombre != "" {
		u.Nombre, err = security.Encrypt(u.Nombre, key)
		if err != nil {
			return fmt.Errorf("error encriptando nombre: %w", err)
		}
	}

	// Encriptar teléfono
	if u.Telefono != "" {
		u.Telefono, err = security.Encrypt(u.Telefono, key)
		if err != nil {
			return fmt.Errorf("error encriptando telefono: %w", err)
		}
	}

	return nil
}

// AfterLoad desencripta todos los datos sensibles al leer de la BD
func (u *UsuarioEntity) AfterLoad(key []byte) error {
	// Desencriptar email
	if u.Email != "" {
		decrypted, err := security.Decrypt(u.Email, key)
		if err != nil {
			log.Printf("[USER] Email no cifrado para usuario %s, se deja como está", u.ID)
		} else {
			u.Email = decrypted
		}
	}

	// Desencriptar nombre
	if u.Nombre != "" {
		decrypted, err := security.Decrypt(u.Nombre, key)
		if err != nil {
			log.Printf("[USER] Nombre no cifrado para usuario %s, se deja como está", u.ID)
		} else {
			u.Nombre = decrypted
		}
	}

	// Desencriptar teléfono
	if u.Telefono != "" {
		decrypted, err := security.Decrypt(u.Telefono, key)
		if err != nil {
			log.Printf("[USER] Teléfono no cifrado para usuario %s, se deja como está", u.ID)
		} else {
			u.Telefono = decrypted
		}
	}

	return nil
}

func DecodeEncryptionKey(b64Key string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(b64Key)
	if err != nil {
		return nil, fmt.Errorf("ENCRYPTION_KEY no es base64 válido: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY debe decodificar a 32 bytes (AES-256), obtenidos: %d", len(key))
	}
	return key, nil
}

type NotificacionEntity struct {
	ID           string
	UserID       string
	Tipo         string
	ReporteID    *string
	Latitud      float64
	Longitud     float64
	NotaVoz      string
	RutaID       string
	Mensaje      string
	Leida        bool
	FechaEnvio   time.Time
	FechaLectura *time.Time
}