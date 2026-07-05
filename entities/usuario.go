package entities

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"time"
)

// UsuarioEntity representa 1:1 la tabla `usuarios` en la base de datos.
// Los campos coinciden exactamente con las columnas de PostgreSQL.
type UsuarioEntity struct {
	ID           string
	Email        string
	PasswordHash string
	Nombre       string
	Tipo         string
	Telefono     string // Almacenado cifrado en BD, descifrado en memoria
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
    encrypted, err := encrypt(u.Telefono, key)
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
    decrypted, err := decrypt(u.Telefono, key)
    if err != nil {
        return err
    }
    u.Telefono = decrypted
    return nil
}
// =============================================================
// Funciones internas de cifrado AES-256-GCM
// =============================================================

// encryptAES cifra plaintext con AES-256-GCM y devuelve el resultado en base64.
func encryptAES(plaintext string, key []byte) (string, error) {
	// La clave debe ser de 32 bytes para AES-256.
	if len(key) != 32 {
		return "", fmt.Errorf("la clave AES debe ser de 32 bytes, recibidos: %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Nonce aleatorio de 12 bytes (estándar GCM)
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Cifrar: el nonce se prepone al ciphertext para poder descifrarlo después.
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptAES descifra un valor base64 cifrado con AES-256-GCM y devuelve el texto plano.
func decryptAES(encoded string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("la clave AES debe ser de 32 bytes, recibidos: %d", len(key))
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 inválido: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext demasiado corto")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("error descifrando: %w", err)
	}

	return string(plaintext), nil
}

// DecodeEncryptionKey decodifica la clave de cifrado desde base64 a []byte de 32 bytes.
// Debe llamarse una sola vez al iniciar la aplicación.
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
