package viaje

import (
	"fmt"
	"strings"
)

func ValidateIniciarViaje(req *IniciarViajeRequest) error {
	req.RutaID = strings.TrimSpace(req.RutaID)
	req.PolylineRuta = strings.TrimSpace(req.PolylineRuta)

	if req.RutaID == "" {
		return fmt.Errorf("el campo 'ruta_id' es requerido")
	}

	if req.PolylineRuta == "" {
		return fmt.Errorf("el campo 'polyline_ruta' es requerido")
	}

	if req.OrigenLat == 0 && req.OrigenLon == 0 {
		return fmt.Errorf("las coordenadas de origen (origen_lat, origen_lon) son requeridas")
	}
	if req.OrigenLat < -90 || req.OrigenLat > 90 {
		return fmt.Errorf("la latitud de origen debe estar entre -90 y 90")
	}
	if req.OrigenLon < -180 || req.OrigenLon > 180 {
		return fmt.Errorf("la longitud de origen debe estar entre -180 y 180")
	}

	if req.DestinoLat == 0 && req.DestinoLon == 0 {
		return fmt.Errorf("las coordenadas de destino (destino_lat, destino_lon) son requeridas")
	}
	if req.DestinoLat < -90 || req.DestinoLat > 90 {
		return fmt.Errorf("la latitud de destino debe estar entre -90 y 90")
	}
	if req.DestinoLon < -180 || req.DestinoLon > 180 {
		return fmt.Errorf("la longitud de destino debe estar entre -180 y 180")
	}

	return nil
}

func ValidateFinalizarViaje(req *FinalizarViajeRequest) error {
	req.ViajeID = strings.TrimSpace(req.ViajeID)
	if req.ViajeID == "" {
		return fmt.Errorf("el campo 'viaje_id' es requerido")
	}
	
	if len(req.ViajeID) != 36 {
		return fmt.Errorf("el campo 'viaje_id' debe ser un UUID válido de 36 caracteres")
	}

	return nil
}
