package services

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"saferoute/entities"
	"saferoute/models"
	"saferoute/repository"
)

type AuthService struct {
	usuarioRepo   *repository.UsuarioRepository
	encryptionKey []byte
	jwtSecret     string
}

func NewAuthService(repo *repository.UsuarioRepository, encryptionKey []byte, jwtSecret string) *AuthService {
	return &AuthService{
		usuarioRepo:   repo,
		encryptionKey: encryptionKey,
		jwtSecret:     jwtSecret,
	}
}

func (s *AuthService) Login(req models.LoginRequest) (models.AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	usuario, err := s.usuarioRepo.FindByEmail(email)
	if err != nil {
		return models.AuthResponse{}, fmt.Errorf("credenciales inválidas")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(usuario.PasswordHash), []byte(req.Password)); err != nil {
		return models.AuthResponse{}, fmt.Errorf("credenciales inválidas")
	}

	token, err := generateJWT(usuario.ID, usuario.Email, usuario.Tipo, s.jwtSecret)
	if err != nil {
		return models.AuthResponse{}, fmt.Errorf("error generando token")
	}

	return models.AuthResponse{
		Token:  token,
		Nombre: usuario.Nombre,
		Tipo:   usuario.Tipo,
		Email:  usuario.Email,
		UserID: usuario.ID,
	}, nil
}

func (s *AuthService) Register(req models.RegisterRequest) (models.AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	nombre := strings.TrimSpace(req.Nombre)
	telefono := strings.TrimSpace(req.Telefono)

	if email == "" || req.Password == "" || nombre == "" {
		return models.AuthResponse{}, fmt.Errorf("email, password y nombre son requeridos")
	}

	existente, _ := s.usuarioRepo.FindByEmail(email)
	if existente != nil {
		return models.AuthResponse{}, fmt.Errorf("el email ya está registrado")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return models.AuthResponse{}, fmt.Errorf("error procesando contraseña")
	}

	tipo := "conductor"

	log.Printf("[AUTH] Registrando usuario - Email: %s, Teléfono: '%s'", email, telefono)

	entity := &entities.UsuarioEntity{
		Email:        email,
		PasswordHash: string(hashedPassword),
		Nombre:       nombre,
		Tipo:         tipo,
		Telefono:     telefono,
	}

	userID, err := s.usuarioRepo.Create(entity)
	if err != nil {
		log.Printf("[AUTH] Error creando usuario: %v", err)
		return models.AuthResponse{}, fmt.Errorf("error al crear usuario")
	}

	log.Printf("[AUTH] Usuario creado - ID: %s", userID)

	token, err := generateJWT(userID, email, tipo, s.jwtSecret)
	if err != nil {
		return models.AuthResponse{}, fmt.Errorf("error generando token")
	}

	return models.AuthResponse{
		Token:  token,
		Nombre: nombre,
		Tipo:   tipo,
		Email:  email,
		UserID: userID,
	}, nil
}

func generateJWT(userID, email, tipo, jwtSecret string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"tipo":    tipo,
		"exp":     time.Now().Add(72 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", fmt.Errorf("error firmando token: %w", err)
	}

	return tokenString, nil
}

func (s *AuthService) RegisterConductor(email, password, nombre, telefono string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	nombre = strings.TrimSpace(nombre)
	telefono = strings.TrimSpace(telefono)

	if email == "" || password == "" || nombre == "" {
		return "", fmt.Errorf("email, password y nombre requeridos")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("error procesando contraseña")
	}

	u := &entities.UsuarioEntity{
		Email:        email,
		PasswordHash: string(hashedPassword),
		Nombre:       nombre,
		Tipo:         "conductor",
		Telefono:     telefono,
	}

	id, err := s.usuarioRepo.Create(u)
	if err != nil {
		return "", fmt.Errorf("el email ya está registrado")
	}
	return id, nil
}
