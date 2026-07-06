package services

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"saferoute/models"
	"saferoute/repository"
)

type ViajeService struct {
	viajeRepo   *repository.ViajeRepository
	usuarioRepo *repository.UsuarioRepository
}

func NewViajeService(viajeRepo *repository.ViajeRepository, usuarioRepo *repository.UsuarioRepository) *ViajeService {
	return &ViajeService{
		viajeRepo:   viajeRepo,
		usuarioRepo: usuarioRepo,
	}
}

// IniciarViaje inicia un nuevo viaje para el usuario
func (s *ViajeService) IniciarViaje(userID string, req models.IniciarViajeRequest) (string, error) {
	log.Print("Llamando 2")
	log.Print("Datos: ", req)
	if req.PolylineRuta == "" {
		return "", fmt.Errorf("la polyline de la ruta es requerida")
	}

	// 1. Decodificar la polilínea en coordenadas [lat, lon]
	coords, err := DecodePolyline(req.PolylineRuta)
	if err != nil {
		return "", fmt.Errorf("error decodificando polyline: %w", err)
	}

	// 2. Convertir coordenadas a formato WKT LINESTRING para PostGIS
	wkt := coordsToWKT(coords)

	// 3. Finalizar automáticamente cualquier viaje activo previo del usuario
	viajeActivo, err := s.viajeRepo.FindActiveByUserID(userID)
	if err == nil && viajeActivo != nil {
		log.Printf("🔄 Finalizando viaje activo anterior (%s) para el usuario %s", viajeActivo.ID, userID)
		s.viajeRepo.UpdateEstado(viajeActivo.ID, "cancelado")
	}

	// 4. Crear la estructura del viaje
	nuevoViaje := &models.Viaje{
		UserID:       userID,
		RutaID:       req.RutaID,
		OrigenLat:    req.OrigenLat,
		OrigenLon:    req.OrigenLon,
		DestinoLat:   req.DestinoLat,
		DestinoLon:   req.DestinoLon,
		PolylineRuta: req.PolylineRuta,
		Estado:       "activo",
	}

	// 5. Guardar en base de datos
	id, err := s.viajeRepo.Create(nuevoViaje, wkt)
	if err != nil {
		return "", err
	}

	log.Print("Llamando final")

	return id, nil
}

// FinalizarViaje finaliza un viaje activo.
// Si el conductor se encuentra lejos del destino (> 50m), requiere validación con su contraseña.
// AHORA RETORNA el estado final: "finalizado" o "cancelado"
func (s *ViajeService) FinalizarViaje(userID string, req models.FinalizarViajeRequest) (string, error) {
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

	// Obtener la última coordenada reportada del conductor
	lastLat, lastLon, found, err := s.viajeRepo.GetLastCoordinate(viaje.ID)
	if err != nil {
		return "", fmt.Errorf("error obteniendo última coordenada: %w", err)
	}

	// Distancia al destino
	distDestino := 99999.0
	if found {
		distDestino = HaversineDistance(lastLat, lastLon, viaje.DestinoLat, viaje.DestinoLon)
	} else {
		// Si no hay historial de coordenadas, usamos el origen del viaje para calcular
		distDestino = HaversineDistance(viaje.OrigenLat, viaje.OrigenLon, viaje.DestinoLat, viaje.DestinoLon)
	}

	// Si está a más de 50 metros del destino, consideramos que es una finalización anticipada (o alerta)
	// y requerimos autenticar la contraseña del usuario
	if distDestino > 50.0 {
		if strings.TrimSpace(req.Password) == "" {
			return "", fmt.Errorf("se requiere ingresar su contraseña para confirmar la finalización anticipada de la ruta por motivos de seguridad")
		}

		// Buscar perfil del usuario para obtener el hash de la contraseña
		usuario, err := s.usuarioRepo.FindByID(userID)
		if err != nil {
			return "", fmt.Errorf("error consultando credenciales de usuario: %w", err)
		}

		// Validar contraseña
		if err := bcrypt.CompareHashAndPassword([]byte(usuario.PasswordHash), []byte(req.Password)); err != nil {
			return "", fmt.Errorf("contraseña incorrecta. No se puede finalizar la ruta")
		}

		log.Printf("⚠️ Viaje %s finalizado de forma anticipada por conductor %s mediante validación de contraseña", viaje.ID, userID)
		return "cancelado", s.viajeRepo.UpdateEstado(viaje.ID, "cancelado")
	}

	log.Printf("✅ Viaje %s finalizado exitosamente (llegada al destino) por conductor %s", viaje.ID, userID)
	return "finalizado", s.viajeRepo.UpdateEstado(viaje.ID, "finalizado")
}

// ActualizarUbicacionViaje actualiza el heartbeat y registra la coordenada.
// Adicionalmente, verifica desvíos (> 100m) y gestiona los cambios de estado correspondientes.
func (s *ViajeService) ActualizarUbicacionViaje(userID string, lat, lon, vel float64) (nuevoEstado string, alertaDesvio bool, err error) {
	viaje, err := s.viajeRepo.FindActiveByUserID(userID)
	if err != nil {
		if err == sql.ErrNoRows {
			// No hay viaje activo, no retornamos error, simplemente no hacemos nada
			return "", false, nil
		}
		return "", false, err
	}

	// Registrar punto y consultar distancias con PostGIS
	distDesvio, _, err := s.viajeRepo.UpdateHeartbeat(viaje.ID, lat, lon, vel)
	if err != nil {
		return viaje.Estado, false, err
	}

	alertaDesvio = false
	estadoActual := viaje.Estado

	// Si el desvío es mayor a 100 metros y el estado no estaba marcado como desviado
	if distDesvio > 100.0 {
		if viaje.Estado != "desviado" {
			estadoActual = "desviado"
			err = s.viajeRepo.UpdateEstado(viaje.ID, "desviado")
			if err == nil {
				alertaDesvio = true
				log.Printf("🚨 Alerta de Desvío detectada para el viaje %s (Conductor: %s). Desviado %.1fm", viaje.ID, userID, distDesvio)
			}
		}
	} else {
		// Si vuelve a la ruta segura (< 100m) y estaba marcado como desviado, restablecer a activo
		if viaje.Estado == "desviado" {
			estadoActual = "activo"
			err = s.viajeRepo.UpdateEstado(viaje.ID, "activo")
			if err == nil {
				log.Printf("🔄 Conductor %s regresó a la ruta segura para el viaje %s", userID, viaje.ID)
			}
		}
	}

	return estadoActual, alertaDesvio, nil
}

// GetActiveViaje obtiene el viaje activo actual de un conductor
func (s *ViajeService) GetActiveViaje(userID string) (*models.Viaje, error) {
	viaje, err := s.viajeRepo.FindActiveByUserID(userID)
	if err != nil {
		return nil, err
	}
	return viaje, nil
}

// GetActiveViajesAdmin obtiene todos los viajes activos (para el admin dashboard)
func (s *ViajeService) GetActiveViajesAdmin() ([]models.ViajeActivoAdmin, error) {
	return s.viajeRepo.FindAllActive()
}

// =========================================================================
// Funciones Auxiliares
// =========================================================================

// DecodePolyline decodifica un string de encoded polyline (Google Directions) a una lista de puntos [lat, lon]
func DecodePolyline(encoded string) ([][2]float64, error) {
	var coords [][2]float64
	index, length := 0, len(encoded)
	lat, lng := 0, 0

	for index < length {
		// Latitud
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

		// Longitud
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

// coordsToWKT convierte un listado de coordenadas a un WKT LINESTRING (ej. "LINESTRING(lon1 lat1, lon2 lat2...)")
func coordsToWKT(coords [][2]float64) string {
	wkt := "LINESTRING("
	for i, pt := range coords {
		if i > 0 {
			wkt += ", "
		}
		// Formato de PostGIS: X Y (Longitude Latitude)
		wkt += fmt.Sprintf("%.8f %.8f", pt[1], pt[0])
	}
	wkt += ")"
	return wkt
}

// HaversineDistance calcula la distancia en metros entre dos puntos geográficos
func HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // Radio de la Tierra en metros
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}