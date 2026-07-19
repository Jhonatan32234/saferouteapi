package user

import (
	"time"

	"saferoute/internal/security"
)

type UsuarioEntity struct {
	ID           string
	Email        string
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
    if u.Telefono == "" {
        return nil
    }
    encrypted, err := security.Encrypt(u.Telefono, key)
    if err != nil {
        return err
    }
    u.Telefono = encrypted
    return nil
}

func (u *UsuarioEntity) AfterLoad(key []byte) error {
    if u.Telefono == "" {
        return nil
    }
    decrypted, err := security.Decrypt(u.Telefono, key)
    if err != nil {
        return err
    }
    u.Telefono = decrypted
    return nil
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
