package handlers

import (
    "encoding/json"
    "net/http"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "golang.org/x/crypto/bcrypt"
    
    "saferoute/database"
    "saferoute/models"
)


func LoginHandler(jwtSecret string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req models.LoginRequest
        
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            writeError(w, http.StatusBadRequest, "datos de entrada inválidos")
            return
        }

        // Validación básica
        if req.Email == "" || req.Password == "" {
            writeError(w, http.StatusBadRequest, "email y contraseña requeridos")
            return
        }

        // Consulta preparada (OWASP A03)
        var id, hash, nombre, tipo string
        err := database.DB.QueryRow(
            "SELECT id, password_hash, nombre, tipo FROM usuarios WHERE email = $1",
            req.Email,
        ).Scan(&id, &hash, &nombre, &tipo)

        if err != nil {
            writeError(w, http.StatusUnauthorized, "credenciales inválidas")
            return
        }

        // Verificar contraseña con bcrypt
        if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
            writeError(w, http.StatusUnauthorized, "credenciales inválidas")
            return
        }

        // Generar JWT
        token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
            "user_id": id,
            "nombre":  nombre,
            "tipo":    tipo,
            "exp":     time.Now().Add(24 * time.Hour).Unix(),
            "iat":     time.Now().Unix(),
        })

        tokenString, err := token.SignedString([]byte(jwtSecret))
        if err != nil {
            writeError(w, http.StatusInternalServerError, "error generando token")
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(models.AuthResponse{
            Token:     tokenString,
            ExpiresIn: 86400,
            UserID:    id,
            Nombre:    nombre,
            Tipo:      tipo,
        })
    }
}

func RegisterHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req models.RegisterRequest
        
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            writeError(w, http.StatusBadRequest, "datos de entrada inválidos")
            return
        }

        // Validaciones
        if req.Email == "" || req.Password == "" || req.Nombre == "" {
            writeError(w, http.StatusBadRequest, "todos los campos son requeridos")
            return
        }

        if req.Tipo != "conductor" && req.Tipo != "admin" {
            writeError(w, http.StatusBadRequest, "tipo debe ser 'conductor' o 'admin'")
            return
        }

        // Hash de contraseña (nunca texto plano)
        hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
        if err != nil {
            writeError(w, http.StatusInternalServerError, "error procesando contraseña")
            return
        }

        // Consulta preparada
        var id string
        err = database.DB.QueryRow(
            `INSERT INTO usuarios (email, password_hash, nombre, tipo) 
             VALUES ($1, $2, $3, $4) 
             RETURNING id`,
            req.Email, string(hashedPassword), req.Nombre, req.Tipo,
        ).Scan(&id)

        if err != nil {
            writeError(w, http.StatusConflict, "el email ya está registrado")
            return
        }

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(map[string]string{
            "id":     id,
            "status": "creado",
        })
    }
}

func writeError(w http.ResponseWriter, code int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(models.ErrorResponse{
        Error: message,
        Code:  code,
    })
}