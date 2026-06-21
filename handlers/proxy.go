package handlers

import (
    "io"
    "net/http"
)

func ProxyHandler(targetURL string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Crear la petición al servicio interno
        req, err := http.NewRequest(r.Method, targetURL, r.Body)
        if err != nil {
            writeError(w, http.StatusInternalServerError, "error creando proxy")
            return
        }

        // Copiar headers
        for key, values := range r.Header {
            for _, value := range values {
                req.Header.Add(key, value)
            }
        }

        // Ejecutar
        client := &http.Client{}
        resp, err := client.Do(req)
        if err != nil {
            writeError(w, http.StatusBadGateway, "servicio no disponible")
            return
        }
        defer resp.Body.Close()

        // Copiar respuesta
        for key, values := range resp.Header {
            for _, value := range values {
                w.Header().Add(key, value)
            }
        }
        w.WriteHeader(resp.StatusCode)
        io.Copy(w, resp.Body)
    }
}