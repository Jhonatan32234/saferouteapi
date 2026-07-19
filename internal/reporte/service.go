package reporte

import (
	"fmt"
)

type UserRepository interface {
	SubscribeRuta(userID string, rutaID string) error
}

type MotorService interface {
	SyncReporteCreado(reporte ReporteResponse)
	SyncReporteValidado(reporteID string, vigente bool)
}

type Service interface {
	Create(req ReporteRequest, userID string) (ReporteResponse, error)
	GetAll(tipo string, vigenteStr string, limit int, offset int) ([]ReporteResponse, error)
	GetByID(id string) (ReporteResponse, error)
	Validar(id string, vigente bool) error
	GetCercanos(lat, lon, radioKm float64, limit int) ([]ReporteResponse, error)
	GetEstadisticas() (ReporteEstadisticas, error)
}

type service struct {
	reporteRepo Repository
	userRepo    UserRepository
	motorSvc    MotorService
}

func NewService(repo Repository, userRepo UserRepository, motorSvc MotorService) Service {
	return &service{
		reporteRepo: repo,
		userRepo:    userRepo,
		motorSvc:    motorSvc,
	}
}

func (s *service) Create(req ReporteRequest, userID string) (ReporteResponse, error) {
	entity := &ReporteEntity{
		UserID:   userID,
		Tipo:     req.Tipo,
		Latitud:  req.Latitud,
		Longitud: req.Longitud,
		NotaVoz:  req.NotaVoz,
		RutaID:   req.RutaID,
	}

	created, err := s.reporteRepo.Create(entity)
	if err != nil {
		return ReporteResponse{}, fmt.Errorf("error al guardar el reporte: %w", err)
	}

	if userID != "" && req.RutaID != "" && userID != "anonimo" {
		go func() {
			_ = s.userRepo.SubscribeRuta(userID, req.RutaID)
		}()
	}

	resp := entityToReporteResponse(created)
	
	// Sincronizar con motores en segundo plano
	if s.motorSvc != nil {
		go s.motorSvc.SyncReporteCreado(resp)
	}

	return resp, nil
}

func (s *service) GetAll(tipo string, vigenteStr string, limit int, offset int) ([]ReporteResponse, error) {
	var vigenteFilter *bool
	if vigenteStr == "true" {
		v := true
		vigenteFilter = &v
	} else if vigenteStr == "false" {
		v := false
		vigenteFilter = &v
	}

	entities, err := s.reporteRepo.FindAll(tipo, vigenteFilter, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("error consultando reportes: %w", err)
	}

	reportes := make([]ReporteResponse, 0, len(entities))
	for _, e := range entities {
		reportes = append(reportes, entityToReporteResponse(&e))
	}
	return reportes, nil
}

func (s *service) GetByID(id string) (ReporteResponse, error) {
	entity, err := s.reporteRepo.FindByID(id)
	if err != nil {
		return ReporteResponse{}, fmt.Errorf("reporte no encontrado")
	}
	return entityToReporteResponse(entity), nil
}

func (s *service) Validar(id string, vigente bool) error {
	if err := s.reporteRepo.Validar(id, vigente); err != nil {
		return fmt.Errorf("error validando reporte: %w", err)
	}

	// Sincronizar estado en segundo plano
	if s.motorSvc != nil {
		go s.motorSvc.SyncReporteValidado(id, vigente)
	}

	return nil
}

func (s *service) GetCercanos(lat, lon, radioKm float64, limit int) ([]ReporteResponse, error) {
	if radioKm <= 0 {
		radioKm = 5.0
	}
	if limit <= 0 {
		limit = 20
	}

	entities, err := s.reporteRepo.FindCercanos(lat, lon, radioKm, limit)
	if err != nil {
		return nil, fmt.Errorf("error buscando reportes cercanos: %w", err)
	}

	reportes := make([]ReporteResponse, 0, len(entities))
	for _, e := range entities {
		reportes = append(reportes, entityToReporteResponse(&e))
	}
	return reportes, nil
}

func (s *service) GetEstadisticas() (ReporteEstadisticas, error) {
	return s.reporteRepo.GetEstadisticas()
}

func entityToReporteResponse(e *ReporteEntity) ReporteResponse {
	return ReporteResponse{
		ID:             e.ID,
		Tipo:           e.Tipo,
		Latitud:        e.Latitud,
		Longitud:       e.Longitud,
		NotaVoz:        e.NotaVoz,
		RutaID:         e.RutaID,
		Timestamp:      e.Timestamp,
		Vigente:        e.Vigente,
		Confirmaciones: e.Confirmaciones,
	}
}
