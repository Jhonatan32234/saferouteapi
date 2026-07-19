package user

import (
	"time"
)

type Service interface {
	GetProfile(userID string) (UserProfile, error)
	UpdateProfile(userID string, req UpdateProfileRequest) error
	
	// Destinos
	SaveDestino(userID string, req DestinoRecienteRequest) error
	GetDestinos(userID string, limit int) ([]DestinoRecienteResponse, error)
	DeleteDestino(userID string, destinoID string) error

	// Zonas
	UpsertZonas(userID string, zonas []ZonaRequest) error
	GetZonas(userID string) ([]ZonaUsuario, error)

	// Suscripciones
	SubscribeRuta(userID string, rutaID string) error
	UnsubscribeRuta(userID string, rutaID string) error
	GetSubscriptions(userID string) ([]SuscripcionRuta, error)

	// Notificaciones
	GetNotifications(userID string, page, limit int, soloNoLeidas bool) (NotificacionHistorialResponse, error)
	MarkNotification(userID string, notifID string, leida bool) error
	MarkAllNotificationsRead(userID string) error
	SyncNotifications(userID string, ultimaSincronizacion time.Time) ([]NotificacionHistorial, error)

	// Conductores interno
	ListConductors() ([]map[string]interface{}, error)
}

type userService struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &userService{repo: repo}
}

func (s *userService) ListConductors() ([]map[string]interface{}, error) {
	return s.repo.ListConductors()
}

func (s *userService) GetProfile(userID string) (UserProfile, error) {
	usuario, err := s.repo.FindByID(userID)
	if err != nil {
		return UserProfile{}, err
	}

	go func() {
		_ = s.repo.UpdateLastAccess(userID)
	}()

	return entityToUserProfile(usuario), nil
}

func (s *userService) UpdateProfile(userID string, req UpdateProfileRequest) error {
	entity := &UsuarioEntity{
		ID:       userID,
		Nombre:   req.Nombre,
		Telefono: req.Telefono,
		Email:    req.Email,
	}

	return s.repo.Update(entity)
}

func (s *userService) SaveDestino(userID string, req DestinoRecienteRequest) error {
	return s.repo.SaveDestino(userID, req.Nombre, req.Lat, req.Lon)
}

func (s *userService) GetDestinos(userID string, limit int) ([]DestinoRecienteResponse, error) {
	return s.repo.GetDestinos(userID, limit)
}

func (s *userService) DeleteDestino(userID string, destinoID string) error {
	return s.repo.DeleteDestino(userID, destinoID)
}

func (s *userService) UpsertZonas(userID string, zonas []ZonaRequest) error {
	return s.repo.UpsertZonas(userID, zonas)
}

func (s *userService) GetZonas(userID string) ([]ZonaUsuario, error) {
	return s.repo.GetZonas(userID)
}

func (s *userService) SubscribeRuta(userID string, rutaID string) error {
	return s.repo.SubscribeRuta(userID, rutaID)
}

func (s *userService) UnsubscribeRuta(userID string, rutaID string) error {
	return s.repo.UnsubscribeRuta(userID, rutaID)
}

func (s *userService) GetSubscriptions(userID string) ([]SuscripcionRuta, error) {
	return s.repo.GetSubscriptions(userID)
}

func (s *userService) GetNotifications(userID string, page, limit int, soloNoLeidas bool) (NotificacionHistorialResponse, error) {
	list, err := s.repo.GetNotifications(userID, page, limit, soloNoLeidas)
	if err != nil {
		return NotificacionHistorialResponse{}, err
	}

	total, err := s.repo.CountNotifications(userID, soloNoLeidas)
	if err != nil {
		total = len(list)
	}

	noLeidas, err := s.repo.CountUnreadNotifications(userID)
	if err != nil {
		noLeidas = 0
	}

	totalPaginas := (total + limit - 1) / limit
	if totalPaginas == 0 {
		totalPaginas = 1
	}

	return NotificacionHistorialResponse{
		Notificaciones: list,
		Total:          total,
		NoLeidas:       noLeidas,
		Pagina:         page,
		TotalPaginas:   totalPaginas,
	}, nil
}

func (s *userService) MarkNotification(userID string, notifID string, leida bool) error {
	return s.repo.MarkNotification(userID, notifID, leida)
}

func (s *userService) MarkAllNotificationsRead(userID string) error {
	return s.repo.MarkAllNotificationsRead(userID)
}

func (s *userService) SyncNotifications(userID string, ultimaSincronizacion time.Time) ([]NotificacionHistorial, error) {
	return s.repo.SyncNotifications(userID, ultimaSincronizacion)
}

func entityToUserProfile(u *UsuarioPerfilConEstadisticas) UserProfile {
	profile := UserProfile{
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
