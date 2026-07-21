package motor

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"saferoute/internal/common"

	"github.com/gorilla/mux"
)

type RutasRequest struct {
	OrigenLat  float64 `json:"origen_lat"`
	OrigenLon  float64 `json:"origen_lon"`
	DestinoLat float64 `json:"destino_lat"`
	DestinoLon float64 `json:"destino_lon"`
}

type RutaResponse struct {
	ID                 string    `json:"id"`
	Nombre             string    `json:"nombre"`
	DistanciaKM        float64   `json:"distancia_km"`
	TiempoMinutos      int       `json:"tiempo_minutos"`
	Seguridad          string    `json:"seguridad"`
	RiesgoCombinado    float64   `json:"riesgo_combinado"`
	ClustersAtravesados []int    `json:"clusters_atravesados"`
	Polyline           string    `json:"polyline"`
}

type RutasResponse struct {
	Rutas       []RutaResponse `json:"rutas"`
	Recomendada string         `json:"recomendada"`
}

type Handler struct {
	motorRutasURL string
}

func NewHandler(motorRutasURL string) *Handler {
	return &Handler{motorRutasURL: motorRutasURL}
}

func (h *Handler) GetRutasHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			common.WriteError(w, http.StatusBadRequest, "error leyendo datos de entrada")
			return
		}
		defer r.Body.Close()

		var req RutasRequest
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		if req.OrigenLat == 0 || req.OrigenLon == 0 || req.DestinoLat == 0 || req.DestinoLon == 0 {
			common.WriteError(w, http.StatusBadRequest, "todas las coordenadas son requeridas")
			return
		}

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

		log.Printf("Llamando al motor de rutas: %s/rutas/calcular", h.motorRutasURL)

		resp, err := http.Post(
			h.motorRutasURL+"/rutas/calcular",
			"application/json",
			bytes.NewBuffer(motorBody),
		)
		if err != nil {
			log.Printf("ERROR motor de rutas: %v", err)
			w.Header().Set("X-Data-Source", "error")
			common.WriteError(w, http.StatusServiceUnavailable, "motor de rutas no disponible")
			return
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			common.WriteError(w, http.StatusInternalServerError, "error leyendo respuesta del motor")
			return
		}

		log.Printf("Motor de rutas respondió: %d bytes", len(respBody))

		if resp.StatusCode != 200 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(resp.StatusCode)
			w.Write(respBody)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Data-Source", "motor-rutas")
		w.Write(respBody)
	}
}

// GetPerfilConductorHandler - GET /api/admin/conductores/{id}/perfil
func (h *Handler) GetPerfilConductorHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        conductorID := vars["id"]

        motorURL := os.Getenv("MOTOR_PREDICCIONES_URL")
        if motorURL == "" {
            motorURL = "http://localhost:8003"
        }

        payload := map[string]string{"conductor_id": conductorID}
        body, _ := json.Marshal(payload)

        resp, err := http.Post(
            motorURL+"/predicciones/perfil",
            "application/json",
            bytes.NewBuffer(body),
        )
        if err != nil {
            common.WriteError(w, http.StatusServiceUnavailable, "motor de predicciones no disponible")
            return
        }
        defer resp.Body.Close()

        var perfil interface{}
        json.NewDecoder(resp.Body).Decode(&perfil)

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(perfil)
    }
}

func ProxyHandler(targetURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var bodyReader io.Reader
		if r.Body != nil {
			bodyBytes, _ := io.ReadAll(r.Body)
			r.Body.Close()
			bodyReader = bytes.NewReader(bodyBytes)
		}
		
		req, err := http.NewRequest(r.Method, targetURL, bodyReader)
		if err != nil {
			common.WriteError(w, http.StatusInternalServerError, "error creando proxy")
			return
		}
		
		for key, values := range r.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
		
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			common.WriteError(w, http.StatusBadGateway, "servicio no disponible: "+err.Error())
			return
		}
		defer resp.Body.Close()
		
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.Header().Set("X-Proxy-Target", targetURL)
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}
