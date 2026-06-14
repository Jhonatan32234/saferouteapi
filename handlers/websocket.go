package handlers

import (
    "encoding/json"
    "net/http"
    "sync"

    "github.com/gorilla/mux"
    "github.com/gorilla/websocket"
)

var (
    upgrader = websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool { return true },
    }
    suscriptores = make(map[string]map[*websocket.Conn]bool)
    subMu        sync.RWMutex
)

func WebSocketHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        rutaID := mux.Vars(r)["ruta_id"]
        
        conn, err := upgrader.Upgrade(w, r, nil)
        if err != nil {
            return
        }
        defer conn.Close()

        subMu.Lock()
        if suscriptores[rutaID] == nil {
            suscriptores[rutaID] = make(map[*websocket.Conn]bool)
        }
        suscriptores[rutaID][conn] = true
        subMu.Unlock()

        // Mantener conexión
        for {
            _, _, err := conn.ReadMessage()
            if err != nil {
                subMu.Lock()
                delete(suscriptores[rutaID], conn)
                subMu.Unlock()
                break
            }
        }
    }
}

func NotificarSuscriptores(rutaID string, data interface{}) {
    subMu.RLock()
    defer subMu.RUnlock()

    if suscriptores[rutaID] == nil {
        return
    }

    msg, _ := json.Marshal(data)
    for conn := range suscriptores[rutaID] {
        conn.WriteMessage(websocket.TextMessage, msg)
    }
}