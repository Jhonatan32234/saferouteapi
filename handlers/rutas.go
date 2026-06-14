package handlers

import (
    "encoding/json"
    "io"
    "net/http"

    "saferoute/models"
)

func GetRutasHandler(motorRutasURL string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req models.RutasRequest
        json.NewDecoder(r.Body).Decode(&req)

        // Llamar al motor de rutas (microservicio Python)
        resp, err := http.Post(
            motorRutasURL+"/rutas/calcular",
            "application/json",
            r.Body,
        )
        if err != nil {
            // Fallback: responder con datos cacheados
            w.Header().Set("X-Data-Source", "cache")
            json.NewEncoder(w).Encode(getCachedRutas())
            return
        }
        defer resp.Body.Close()

        body, _ := io.ReadAll(resp.Body)
        
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("X-Data-Source", "motor-rutas")
        w.Write(body)
    }
}

func getCachedRutas() models.RutasResponse {
    // Datos precargados para modo offline/fallback
    return models.RutasResponse{
        Rutas: []models.RutaResponse{},
        Recomendada: "",
    }
}