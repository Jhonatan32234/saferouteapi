package entities

import (
	"time"
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
    encrypted, err := Encrypt(u.Telefono, key)
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
    decrypted, err := Decrypt(u.Telefono, key)
    if err != nil {
        return err
    }
    u.Telefono = decrypted
    return nil
}

