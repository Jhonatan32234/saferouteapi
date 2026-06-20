package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"saferoute/models"
)

func GetRutasHandler(motorRutasURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Leer el body original
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "error leyendo datos de entrada")
			return
		}
		defer r.Body.Close()

		// Validar que el JSON es válido
		var req models.RutasRequest
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			writeError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		// Validar coordenadas
		if req.OrigenLat == 0 || req.OrigenLon == 0 || req.DestinoLat == 0 || req.DestinoLon == 0 {
			writeError(w, http.StatusBadRequest, "todas las coordenadas son requeridas")
			return
		}

		// Convertir al formato que espera el motor de rutas
		motorReq := map[string]interface{}{
			"origen": map[string]float64{
				"lat": req.OrigenLat,
				"lon": req.OrigenLon,
			},
			"destino": map[string]float64{
				"lat": req.DestinoLat,
				"lon": req.DestinoLon,
			},
		}

		motorBody, _ := json.Marshal(motorReq)

		log.Printf("Llamando al motor de rutas: %s/rutas/calcular", motorRutasURL)

		// Llamar al motor de rutas
		resp, err := http.Post(
			motorRutasURL+"/rutas/calcular",
			"application/json",
			bytes.NewBuffer(motorBody),
		)
		if err != nil {
			log.Printf("ERROR motor de rutas: %v", err)
			w.Header().Set("X-Data-Source", "error")
			writeError(w, http.StatusServiceUnavailable, "motor de rutas no disponible")
			return
		}
		defer resp.Body.Close()

		// Leer respuesta del motor
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "error leyendo respuesta del motor")
			return
		}

		log.Printf("Motor de rutas respondió: %d bytes", len(respBody))

		// Si el motor devuelve error, pasarlo al cliente
		if resp.StatusCode != 200 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(resp.StatusCode)
			w.Write(respBody)
			return
		}

		// Responder con los datos del motor
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Data-Source", "motor-rutas")
		w.Write(respBody)
	}
}

// getCachedRutas ya no se usa, pero la dejamos por si falla el motor
func getCachedRutas() models.RutasResponse {
	return models.RutasResponse{
		Rutas:       []models.RutaResponse{},
		Recomendada: "",
	}
}