package services

import (
	"saferoute/entities"
	"saferoute/models"
	"saferoute/repository"
)

// UserService implementa la lógica de negocio para los perfiles de usuario.
type UserService struct {
	usuarioRepo   *repository.UsuarioRepository
	encryptionKey []byte
}

// NewUserService crea una nueva instancia de UserService.
func NewUserService(repo *repository.UsuarioRepository, encryptionKey []byte) *UserService {
	return &UserService{
		usuarioRepo:   repo,
		encryptionKey: encryptionKey,
	}
}

// GetProfile recupera el perfil del usuario descifrado.
func (s *UserService) GetProfile(userID string) (models.UserProfile, error) {
	usuario, err := s.usuarioRepo.FindByID(userID)
	if err != nil {
		return models.UserProfile{}, err
	}

	// Actualizar último acceso en background (no bloquea)
	go func() {
		_ = s.usuarioRepo.UpdateLastAccess(userID)
	}()

	return entityToUserProfile(usuario), nil
}

// UpdateProfile actualiza el perfil del usuario ( BeforeSave cifra el teléfono).
func (s *UserService) UpdateProfile(userID string, req models.UpdateProfileRequest) error {
	entity := &entities.UsuarioEntity{
		ID:       userID,
		Nombre:   req.Nombre,
		Telefono: req.Telefono,
		Email:    req.Email,
	}

	return s.usuarioRepo.Update(entity)
}

// entityToUserProfile mapea una UsuarioEntity a un UserProfile DTO de salida.
func entityToUserProfile(u *entities.UsuarioEntity) models.UserProfile {
	profile := models.UserProfile{
		ID:       u.ID,
		Email:    u.Email,
		Nombre:   u.Nombre,
		Tipo:     u.Tipo,
		Telefono: u.Telefono,
	}
	if !u.CreatedAt.IsZero() {
		profile.CreatedAt = u.CreatedAt
	}
	if !u.UpdatedAt.IsZero() {
		profile.UpdatedAt = u.UpdatedAt
	}
	if u.UltimoAcceso != nil {
		profile.UltimoAcceso = *u.UltimoAcceso
	}
	return profile
}
