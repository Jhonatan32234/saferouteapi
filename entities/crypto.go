package entities

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// encrypt cifra un texto plano usando AES-GCM y devuelve el resultado en base64.
func encrypt(plaintext string, key []byte) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// El nonce se antepone al ciphertext
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	
	// Codificar en base64 para almacenamiento seguro
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt descifra un texto cifrado en base64 usando AES-GCM.
func decrypt(encodedCiphertext string, key []byte) (string, error) {
	if encodedCiphertext == "" {
		return "", nil
	}

	// Decodificar de base64
	ciphertext, err := base64.StdEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		return "", err
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
		return "", errors.New("ciphertext demasiado corto")
	}

	// Extraer nonce y ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Descifrar
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}