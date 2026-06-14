package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/gorilla/mux"
    
    "saferoute/database"
    "saferoute/middleware"
    "saferoute/models"
)

func CreateReporteHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req models.ReporteRequest
        
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            writeError(w, http.StatusBadRequest, "datos de entrada inválidos")
            return
        }

        // Validar campos requeridos
        if req.Tipo == "" || req.Latitud == 0 || req.Longitud == 0 || req.RutaID == "" {
            writeError(w, http.StatusBadRequest, "tipo, latitud, longitud y ruta_id son requeridos")
            return
        }

        userID := middleware.GetUserID(r)

        // Consulta preparada
        var reporte models.ReporteResponse
        err := database.DB.QueryRow(
            `INSERT INTO reportes (user_id, tipo, latitud, longitud, nota_voz, ruta_id) 
             VALUES ($1, $2, $3, $4, $5, $6) 
             RETURNING id, tipo, latitud, longitud, COALESCE(nota_voz,''), ruta_id, timestamp, vigente, confirmaciones`,
            userID, req.Tipo, req.Latitud, req.Longitud, req.NotaVoz, req.RutaID,
        ).Scan(
            &reporte.ID, &reporte.Tipo, &reporte.Latitud, &reporte.Longitud,
            &reporte.NotaVoz, &reporte.RutaID, &reporte.Timestamp,
            &reporte.Vigente, &reporte.Confirmaciones,
        )

        if err != nil {
            writeError(w, http.StatusInternalServerError, "error creando reporte")
            return
        }

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(reporte)
    }
}

func GetReportesHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        rows, err := database.DB.Query(
            `SELECT id, tipo, latitud, longitud, COALESCE(nota_voz,''), ruta_id, 
                    timestamp, vigente, confirmaciones 
             FROM reportes 
             WHERE vigente = TRUE 
             ORDER BY timestamp DESC 
             LIMIT 50`,
        )
        if err != nil {
            writeError(w, http.StatusInternalServerError, "error consultando reportes")
            return
        }
        defer rows.Close()

        var reportes []models.ReporteResponse
        for rows.Next() {
            var rep models.ReporteResponse
            rows.Scan(&rep.ID, &rep.Tipo, &rep.Latitud, &rep.Longitud,
                &rep.NotaVoz, &rep.RutaID, &rep.Timestamp,
                &rep.Vigente, &rep.Confirmaciones)
            reportes = append(reportes, rep)
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "reportes": reportes,
            "total":    len(reportes),
        })
    }
}

func ValidarReporteHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        reporteID := mux.Vars(r)["id"]
        
        var accion struct {
            Vigente bool `json:"vigente"`
        }
        json.NewDecoder(r.Body).Decode(&accion)

        if accion.Vigente {
            database.DB.Exec(
                "UPDATE reportes SET confirmaciones = confirmaciones + 1 WHERE id = $1",
                reporteID,
            )
        } else {
            database.DB.Exec(
                "UPDATE reportes SET vigente = FALSE WHERE id = $1",
                reporteID,
            )
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "actualizado"})
    }
}