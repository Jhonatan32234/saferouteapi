package services

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"saferoute/entities"
	"saferoute/models"
	"saferoute/repository"
)

// AuthService contiene la lógica de negocio de autenticación.
// No tiene acceso directo a SQL; delega toda la persistencia al repositorio.
type AuthService struct {
	usuarioRepo   *repository.UsuarioRepository
	encryptionKey []byte
}

// NewAuthService crea una nueva instancia del servicio de autenticación.
func NewAuthService(repo *repository.UsuarioRepository, encryptionKey []byte) *AuthService {
	return &AuthService{
		usuarioRepo:   repo,
		encryptionKey: encryptionKey,
	}
}

// Login autentica al usuario y devuelve un token JWT válido por 24 horas.
// El repositorio se encarga de recuperar y descifrar el teléfono; el servicio
// solo trabaja con la entidad limpia.
func (s *AuthService) Login(req models.LoginRequest, jwtSecret string) (models.AuthResponse, error) {
	// El repositorio recupera el usuario con AfterLoad aplicado
	usuario, err := s.usuarioRepo.FindByEmail(req.Email)
	if err != nil {
		return models.AuthResponse{}, fmt.Errorf("credenciales inválidas")
	}

	// Verificar contraseña con bcrypt
	if err := bcrypt.CompareHashAndPassword([]byte(usuario.PasswordHash), []byte(req.Password)); err != nil {
		return models.AuthResponse{}, fmt.Errorf("credenciales inválidas")
	}

	// Generar JWT con claims de seguridad
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": usuario.ID,
		"nombre":  usuario.Nombre,
		"tipo":    usuario.Tipo,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return models.AuthResponse{}, fmt.Errorf("error generando token")
	}

	return models.AuthResponse{
		Token:     tokenString,
		ExpiresIn: 86400,
		UserID:    usuario.ID,
		Nombre:    usuario.Nombre,
		Tipo:      usuario.Tipo,
	}, nil
}

// Register crea una cuenta nueva con contraseña hasheada y teléfono cifrado.
// Devuelve el UUID del usuario creado.
func (s *AuthService) Register(req models.RegisterRequest) (string, error) {
	// Hash de contraseña con bcrypt (nunca texto plano en BD)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("error procesando contraseña")
	}

	// Construir entidad limpia. BeforeSave se aplicará en el repositorio.
	u := &entities.UsuarioEntity{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Nombre:       req.Nombre,
		Tipo:         req.Tipo,
	}

	// El repositorio aplica BeforeSave → cifra Telefono (vacío en registro público)
	id, err := s.usuarioRepo.Create(u)
	if err != nil {
		return "", fmt.Errorf("el email ya está registrado")
	}
	return id, nil
}

// RegisterConductor crea un conductor con teléfono, invocado solo por admins.
func (s *AuthService) RegisterConductor(email, password, nombre, telefono string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("error procesando contraseña")
	}

	u := &entities.UsuarioEntity{
		Email:        email,
		PasswordHash: string(hashedPassword),
		Nombre:       nombre,
		Tipo:         "conductor",
		Telefono:     telefono, // BeforeSave lo cifrará en el repositorio
	}

	id, err := s.usuarioRepo.Create(u)
	if err != nil {
		return "", fmt.Errorf("el email ya está registrado")
	}
	return id, nil
}
