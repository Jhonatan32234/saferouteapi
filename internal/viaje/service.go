package viaje

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"saferoute/internal/user"
)

type UsuarioRepository interface {
	FindByID(id string) (*user.UsuarioPerfilConEstadisticas, error)
}

type Service interface {
	IniciarViaje(userID string, req IniciarViajeRequest) (string, error)
	FinalizarViaje(userID string, req FinalizarViajeRequest) (string, error)
	ActualizarUbicacionViaje(userID string, lat, lon, vel float64) (nuevoEstado string, alertaDesvio bool, err error)
	GetActiveViaje(userID string) (*Viaje, error)
	GetActiveViajesAdmin() ([]ViajeActivoAdmin, error)
	GetActiveViajesByEmpresa(empresaID string) ([]ViajeActivoAdmin, error)  // ← NUEVO

}

type service struct {
	viajeRepo   Repository
	usuarioRepo UsuarioRepository
}

func NewService(viajeRepo Repository, usuarioRepo UsuarioRepository) Service {
	return &service{
		viajeRepo:   viajeRepo,
		usuarioRepo: usuarioRepo,
	}
}

func (s *service) GetActiveViajesByEmpresa(empresaID string) ([]ViajeActivoAdmin, error) {
    return s.viajeRepo.FindActiveByEmpresa(empresaID)
}


func (s *service) IniciarViaje(userID string, req IniciarViajeRequest) (string, error) {
	if req.PolylineRuta == "" {
		return "", fmt.Errorf("la polyline de la ruta es requerida")
	}

	coords, err := DecodePolyline(req.PolylineRuta)
	if err != nil {
		return "", fmt.Errorf("error decodificando polyline: %w", err)
	}

	wkt := coordsToWKT(coords)

	viajeActivo, err := s.viajeRepo.FindActiveByUserID(userID)
	if err == nil && viajeActivo != nil {
		log.Printf("Finalizando viaje activo anterior (%s) para el usuario %s", viajeActivo.ID, userID)
		s.viajeRepo.UpdateEstado(viajeActivo.ID, "cancelado")
	}

	nuevoViaje := &Viaje{
		UserID:       userID,
		RutaID:       req.RutaID,
		OrigenLat:    req.OrigenLat,
		OrigenLon:    req.OrigenLon,
		DestinoLat:   req.DestinoLat,
		DestinoLon:   req.DestinoLon,
		PolylineRuta: req.PolylineRuta,
		Estado:       "activo",
	}

	id, err := s.viajeRepo.Create(nuevoViaje, wkt)
	if err != nil {
		return "", err
	}

	return id, nil
}

func (s *service) FinalizarViaje(userID string, req FinalizarViajeRequest) (string, error) {
	viaje, err := s.viajeRepo.FindByID(req.ViajeID)
	if err != nil {
		return "", fmt.Errorf("viaje no encontrado: %w", err)
	}

	if viaje.UserID != userID {
		return "", fmt.Errorf("no autorizado para finalizar este viaje")
	}

	if viaje.Estado == "finalizado" || viaje.Estado == "cancelado" {
		return "", fmt.Errorf("el viaje ya ha sido finalizado previamente")
	}

	lastLat, lastLon, found, err := s.viajeRepo.GetLastCoordinate(viaje.ID)
	if err != nil {
		return "", fmt.Errorf("error obteniendo última coordenada: %w", err)
	}

	distDestino := 99999.0
	if found {
		distDestino = HaversineDistance(lastLat, lastLon, viaje.DestinoLat, viaje.DestinoLon)
	} else {
		distDestino = HaversineDistance(viaje.OrigenLat, viaje.OrigenLon, viaje.DestinoLat, viaje.DestinoLon)
	}

	if distDestino > 50.0 {
		if strings.TrimSpace(req.Password) == "" {
			return "", fmt.Errorf("se requiere ingresar su contraseña para confirmar la finalización anticipada de la ruta por motivos de seguridad")
		}

		usuario, err := s.usuarioRepo.FindByID(userID)
		if err != nil {
			return "", fmt.Errorf("error consultando credenciales de usuario: %w", err)
		}

		if err := bcrypt.CompareHashAndPassword([]byte(usuario.PasswordHash), []byte(req.Password)); err != nil {
			return "", fmt.Errorf("contraseña incorrecta. No se puede finalizar la ruta")
		}

		log.Printf("Viaje %s finalizado de forma anticipada por conductor %s mediante validación de contraseña", viaje.ID, userID)
		return "cancelado", s.viajeRepo.UpdateEstado(viaje.ID, "cancelado")
	}

	log.Printf("Viaje %s finalizado exitosamente (llegada al destino) por conductor %s", viaje.ID, userID)
	return "finalizado", s.viajeRepo.UpdateEstado(viaje.ID, "finalizado")
}

func (s *service) ActualizarUbicacionViaje(userID string, lat, lon, vel float64) (nuevoEstado string, alertaDesvio bool, err error) {
	viaje, err := s.viajeRepo.FindActiveByUserID(userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}

	distDesvio, _, err := s.viajeRepo.UpdateHeartbeat(viaje.ID, lat, lon, vel)
	if err != nil {
		return viaje.Estado, false, err
	}

	alertaDesvio = false
	estadoActual := viaje.Estado

	if distDesvio > 100.0 {
		if viaje.Estado != "desviado" {
			estadoActual = "desviado"
			err = s.viajeRepo.UpdateEstado(viaje.ID, "desviado")
			if err == nil {
				alertaDesvio = true
				log.Printf("Alerta de Desvío detectada para el viaje %s (Conductor: %s). Desviado %.1fm", viaje.ID, userID, distDesvio)
			}
		}
	} else {
		if viaje.Estado == "desviado" {
			estadoActual = "activo"
			err = s.viajeRepo.UpdateEstado(viaje.ID, "activo")
			if err == nil {
				log.Printf("Conductor %s regresó a la ruta segura para el viaje %s", userID, viaje.ID)
			}
		}
	}

	return estadoActual, alertaDesvio, nil
}

func (s *service) GetActiveViaje(userID string) (*Viaje, error) {
	viaje, err := s.viajeRepo.FindActiveByUserID(userID)
	if err != nil {
		return nil, err
	}
	return viaje, nil
}

func (s *service) GetActiveViajesAdmin() ([]ViajeActivoAdmin, error) {
	return s.viajeRepo.FindAllActive()
}

func DecodePolyline(encoded string) ([][2]float64, error) {
	var coords [][2]float64
	index, length := 0, len(encoded)
	lat, lng := 0, 0

	for index < length {
		b, shift, result := 0, 0, 0
		for {
			if index >= length {
				return nil, fmt.Errorf("polyline codificada no es válida")
			}
			b = int(encoded[index]) - 63
			index++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		var dLat int
		if (result & 1) != 0 {
			dLat = ^(result >> 1)
		} else {
			dLat = result >> 1
		}
		lat += dLat

		shift, result = 0, 0
		for {
			if index >= length {
				return nil, fmt.Errorf("polyline codificada no es válida")
			}
			b = int(encoded[index]) - 63
			index++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		var dLng int
		if (result & 1) != 0 {
			dLng = ^(result >> 1)
		} else {
			dLng = result >> 1
		}
		lng += dLng

		coords = append(coords, [2]float64{
			float64(lat) / 1e5,
			float64(lng) / 1e5,
		})
	}
	return coords, nil
}

func coordsToWKT(coords [][2]float64) string {
	wkt := "LINESTRING("
	for i, pt := range coords {
		if i > 0 {
			wkt += ", "
		}
		wkt += fmt.Sprintf("%.8f %.8f", pt[1], pt[0])
	}
	wkt += ")"
	return wkt
}

func HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}
