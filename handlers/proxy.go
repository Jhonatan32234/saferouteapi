package handlers

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

// handlers/proxy.go - Versión corregida
func ProxyHandler(targetURL string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Leer el body primero (importante para POST/PUT)
        var bodyReader io.Reader
        if r.Body != nil {
            bodyBytes, _ := io.ReadAll(r.Body)
            r.Body.Close()
            bodyReader = bytes.NewReader(bodyBytes)
        }
        
        req, err := http.NewRequest(r.Method, targetURL, bodyReader)
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
        
        client := &http.Client{Timeout: 15 * time.Second}
        resp, err := client.Do(req)
        if err != nil {
            writeError(w, http.StatusBadGateway, "servicio no disponible: "+err.Error())
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