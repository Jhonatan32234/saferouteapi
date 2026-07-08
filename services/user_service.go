package services

import (
	"saferoute/entities"
	"saferoute/models"
	"saferoute/repository"
)

type UserService struct {
	usuarioRepo   *repository.UsuarioRepository
	encryptionKey []byte
}

func NewUserService(repo *repository.UsuarioRepository, encryptionKey []byte) *UserService {
	return &UserService{
		usuarioRepo:   repo,
		encryptionKey: encryptionKey,
	}
}

func (s *UserService) GetProfile(userID string) (models.UserProfile, error) {
    usuario, err := s.usuarioRepo.FindByID(userID)
    if err != nil {
        return models.UserProfile{}, err
    }

    go func() {
        _ = s.usuarioRepo.UpdateLastAccess(userID)
    }()


    return entityToUserProfile(usuario), nil
}

func (s *UserService) UpdateProfile(userID string, req models.UpdateProfileRequest) error {
	entity := &entities.UsuarioEntity{
		ID:       userID,
		Nombre:   req.Nombre,
		Telefono: req.Telefono,
		Email:    req.Email,
	}

	return s.usuarioRepo.Update(entity)
}

func entityToUserProfile(u *entities.UsuarioPerfilConEstadisticas) models.UserProfile {
    profile := models.UserProfile{
        ID:                  u.ID,
        Email:               u.Email,
        Nombre:              u.Nombre,
        Tipo:                u.Tipo,
        Telefono:            u.Telefono,
        ReportesCreados:     u.ReportesCreados,
        ReportesConfirmados: u.ReportesConfirmados,
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