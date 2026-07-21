# Documentación de Validación de Entradas de Datos — SafeRoute API (Lado del Servidor)

> **Propósito:** Documentar cada tipo de validación de entrada de datos implementada en el backend Go de SafeRoute, mostrando las llamadas a función, bloques de código, librerías y frameworks utilizados.
>
> **Integración:** Este documento complementa el reporte de validación del lado del cliente (Flutter) ya realizado.

---

## Índice

1. [Librerías y Frameworks Utilizados](#1-librerías-y-frameworks-utilizados)
2. [Validación del Lado del Cliente (vista servidor)](#2-validación-del-lado-del-cliente-vista-servidor)
3. [Validación del Lado del Servidor](#3-validación-del-lado-del-servidor)
4. [Validación de Tipo](#4-validación-de-tipo)
5. [Validación de Lógica de Negocio](#5-validación-de-lógica-de-negocio)
6. [Validación de Patrones y Reglas Específicas](#6-validación-de-patrones-y-reglas-específicas)
7. [Validación Cruzada](#7-validación-cruzada)
8. [Validación Contextual](#8-validación-contextual)
9. [Sanitización de Entrada](#9-sanitización-de-entrada)
10. [Uso de Librerías y Frameworks de Validación](#10-uso-de-librerías-y-frameworks-de-validación)
11. [Educación y Capacitación del Equipo](#11-educación-y-capacitación-del-equipo)
12. [Gestión de Errores Adecuada](#12-gestión-de-errores-adecuada)

---

## 1. Librerías y Frameworks Utilizados

### 1.1 Dependencias del proyecto (`go.mod`)

```go
require (
    github.com/golang-jwt/jwt/v5 v5.3.1
    github.com/gorilla/mux v1.8.1
    github.com/gorilla/websocket v1.5.3
    github.com/joho/godotenv v1.5.1
    github.com/lib/pq v1.12.3
    github.com/rs/cors v1.11.1
    golang.org/x/crypto v0.53.0
    golang.org/x/time v0.15.0
)
```

| Librería | Función | Origen |
|---|---|---|
| `gorilla/mux` | Enrutador HTTP, extracción de parámetros URL | https://github.com/gorilla/mux |
| `golang-jwt/jwt/v5` | Validación y parseo de tokens JWT Ed25519 | https://github.com/golang-jwt/jwt |
| `lib/pq` | Driver PostgreSQL con consultas parametrizadas (SQL injection prevention) | https://github.com/lib/pq |
| `rs/cors` | Middleware CORS para control de orígenes | https://github.com/rs/cors |
| `golang.org/x/crypto` | Funciones criptográficas (bcrypt, Ed25519) | https://pkg.go.dev/golang.org/x/crypto |
| `golang.org/x/time/rate` | Rate limiting por IP | https://pkg.go.dev/golang.org/x/time/rate |
| `gorilla/websocket` | Conexiones WebSocket seguras | https://github.com/gorilla/websocket |
| `joho/godotenv` | Carga de variables de entorno desde `.env` | https://github.com/joho/godotenv |
| `crypto/ed25519` (stdlib) | Firma y verificación de peticiones internas | Estándar de Go |
| `crypto/aes` + `crypto/cipher` (stdlib) | Cifrado AES-256-GCM para datos sensibles | Estándar de Go |

---

## 2. Validación del Lado del Cliente (vista servidor)

Si bien la validación del lado del cliente se realiza en Flutter, el servidor **no confía** en ella y aplica sus propias validaciones independientes. A continuación se documentan las validaciones que el servidor realiza **análogamente** a las del cliente, como capa de defensa en profundidad.

### 2.1 Validación de Formato

#### Email
Se verifica que el campo contenga el caracter `@` como validación básica de formato.

**Archivo:** `internal/auth/pipe.go`

```go
func ValidateLogin(req *LoginRequest) error {
    req.Email = strings.ToLower(strings.TrimSpace(req.Email))
    if !strings.Contains(req.Email, "@") {
        return fmt.Errorf("el campo 'email' no tiene un formato válido")
    }
    // ...
}

func ValidateRegister(req *RegisterRequest) error {
    req.Email = strings.ToLower(strings.TrimSpace(req.Email))
    // ...
    if !strings.Contains(req.Email, "@") {
        return fmt.Errorf("el campo 'email' no tiene un formato válido")
    }
    // ...
}
```

#### Coordenadas Geográficas
Validación de formato de latitud/longitud (rangos geográficos):

**Archivo:** `internal/reporte/pipe.go` y `internal/viaje/pipe.go`

```go
func ValidateReporte(req *ReporteRequest) error {
    if req.Latitud < -90 || req.Latitud > 90 {
        return fmt.Errorf("la latitud debe estar entre -90 y 90")
    }
    if req.Longitud < -180 || req.Longitud > 180 {
        return fmt.Errorf("la longitud debe estar entre -180 y 180")
    }
    // ...
}
```

```go
func ValidateIniciarViaje(req *IniciarViajeRequest) error {
    if req.OrigenLat < -90 || req.OrigenLat > 90 {
        return fmt.Errorf("la latitud de origen debe estar entre -90 y 90")
    }
    if req.OrigenLon < -180 || req.OrigenLon > 180 {
        return fmt.Errorf("la longitud de origen debe estar entre -180 y 180")
    }
    if req.DestinoLat < -90 || req.DestinoLat > 90 {
        return fmt.Errorf("la latitud de destino debe estar entre -90 y 90")
    }
    if req.DestinoLon < -180 || req.DestinoLon > 180 {
        return fmt.Errorf("la longitud de destino debe estar entre -180 y 180")
    }
    // ...
}
```

#### UUID
Validación de que un `viaje_id` tenga exactamente 36 caracteres (formato UUID):

**Archivo:** `internal/viaje/pipe.go`

```go
func ValidateFinalizarViaje(req *FinalizarViajeRequest) error {
    req.ViajeID = strings.TrimSpace(req.ViajeID)
    if len(req.ViajeID) != 36 {
        return fmt.Errorf("el campo 'viaje_id' debe ser un UUID válido de 36 caracteres")
    }
    // ...
}
```

### 2.2 Validación de Longitud

#### Contraseña
Se verifica longitud mínima de 6 caracteres:

**Archivo:** `internal/auth/pipe.go`

```go
func ValidateRegister(req *RegisterRequest) error {
    if len(req.Password) < 6 {
        return fmt.Errorf("la contraseña debe tener al menos 6 caracteres")
    }
    // ...
}
```

#### Nota de Voz (descripción)
Se trunca a 300 caracteres máximo:

**Archivo:** `internal/reporte/pipe.go`

```go
func ValidateReporte(req *ReporteRequest) error {
    req.NotaVoz = strings.TrimSpace(req.NotaVoz)
    if len(req.NotaVoz) > 300 {
        req.NotaVoz = req.NotaVoz[:297] + "..."
    }
    // ...
}
```

### 2.3 Validación de Rango

#### Coordenadas geográficas
Referenciado en [2.1](#coordenadas-geográficas) — se validan rangos de latitud (-90 a 90) y longitud (-180 a 180).

#### Parámetros de paginación (query params)
**Archivo:** `internal/reporte/handler.go`

```go
limit := 50
if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 200 {
    limit = l
}
offset := 0
if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o >= 0 {
    offset = o
}
```

**Archivo:** `internal/user/notificaciones_handler.go`

```go
if l, err := strconv.Atoi(limiteStr); err == nil && l > 0 && l <= 100 {
    limite = l
}
```

**Archivo:** `internal/user/destinos_handler.go`

```go
if l, err := strconv.Atoi(limiteStr); err == nil && l > 0 && l <= 50 {
    limite = l
}
```

#### Radio de búsqueda (km)
**Archivo:** `internal/reporte/handler.go`

```go
radioKm := 30.0
if rv, err := strconv.ParseFloat(r.URL.Query().Get("radio_km"), 64); err == nil && rv > 0 && rv <= 100 {
    radioKm = rv
}
```

### 2.4 Validación de Contenido

#### Campos requeridos vacíos
Se verifica que campos obligatorios no estén vacíos después de aplicar `TrimSpace`:

**Archivo:** `internal/auth/pipe.go`
```go
if req.Email == "" {
    return fmt.Errorf("el campo 'email' es requerido")
}
if strings.TrimSpace(req.Password) == "" {
    return fmt.Errorf("el campo 'password' es requerido")
}
if req.Nombre == "" {
    return fmt.Errorf("el campo 'nombre' es requerido")
}
```

**Archivo:** `internal/reporte/pipe.go`
```go
if req.Tipo == "" {
    return fmt.Errorf("el campo 'tipo' es requerido")
}
if req.Latitud == 0 && req.Longitud == 0 {
    return fmt.Errorf("los campos 'latitud' y 'longitud' son requeridos")
}
```

**Archivo:** `internal/viaje/pipe.go`
```go
if req.RutaID == "" {
    return fmt.Errorf("el campo 'ruta_id' es requerido")
}
if req.PolylineRuta == "" {
    return fmt.Errorf("el campo 'polyline_ruta' es requerido")
}
```

#### Validación de tipo de reporte contra lista blanca
**Archivo:** `internal/reporte/pipe.go`
```go
var tiposPermitidosPipe = map[string]bool{
    "accidente":  true,
    "inundacion": true,
    "bache":      true,
    "derrumbe":   true,
    "sin_luz":    true,
    "niebla":     true,
    "bloqueo":    true,
    "otro":       true,
}

if !tiposPermitidosPipe[req.Tipo] {
    return fmt.Errorf("tipo inválido '%s'. Valores permitidos: ...", req.Tipo)
}
```

#### Validación de tipo de usuario (whitelist)
**Archivo:** `internal/auth/pipe.go`
```go
if req.Tipo != "conductor" {
    return fmt.Errorf("el campo 'tipo' solo puede ser 'conductor' para el registro público")
}
```

### 2.5 Validación de Expresiones Regulares (regex)

Actualmente el backend no utiliza expresiones regulares para validación de entrada. Las validaciones se realizan mediante:
- `strings.Contains()` para verificar presencia de `@` en emails
- Comparaciones de rango numérico para coordenadas
- Verificación de longitud exacta (36 caracteres) para UUIDs
- Mapas de conjunto (`map[string]bool`) para listas blancas (whitelist)

> **Nota:** Se recomienda implementar expresiones regulares para validación más estricta de emails (RFC 5322) y UUIDs completos (formato `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`).

---

## 3. Validación del Lado del Servidor

### 3.1 Validación de Autenticidad

#### Validación de Tokens JWT (Ed25519)
Se verifica la autenticidad de los tokens JWT firmados con Ed25519:

**Archivo:** `internal/middleware/auth.go`

```go
func ValidateToken(tokenString string, pubKey ed25519.PublicKey) (jwt.MapClaims, error) {
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return pubKey, nil
    })
    if err != nil {
        return nil, err
    }
    if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
        return claims, nil
    }
    return nil, fmt.Errorf("invalid token")
}
```

**Librería:** `github.com/golang-jwt/jwt/v5` — verifica la firma digital, el método de firma (Ed25519), y la validez del token (expiración, etc.).

#### Middleware de Autenticación
Verifica el header `Authorization: Bearer <token>` en cada ruta protegida:

**Archivo:** `internal/middleware/auth.go`

```go
func AuthMiddleware(pubKey ed25519.PublicKey) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                http.Error(w, "Authorization header required", http.StatusUnauthorized)
                return
            }
            parts := strings.Split(authHeader, " ")
            if len(parts) != 2 || parts[0] != "Bearer" {
                http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
                return
            }
            tokenString := parts[1]
            claims, err := ValidateToken(tokenString, pubKey)
            if err != nil {
                http.Error(w, "Invalid token", http.StatusUnauthorized)
                return
            }
            // Extraer claims y agregar al contexto
            ctx := r.Context()
            if userID, ok := claims["user_id"].(string); ok && userID != "" {
                ctx = context.WithValue(ctx, UserIDKey, userID)
            }
            // ...
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

#### Firma de Peticiones Internas (HMAC con Ed25519)
Las comunicaciones entre servicios internos se firman para verificar autenticidad y prevenir ataques de repetición:

**Archivo:** `internal/security/signing.go`

```go
func SignRequest(privateKey ed25519.PrivateKey, method, path string, timestamp int64, body []byte) (string, error) {
    data := fmt.Sprintf("%d:%s:%s:%s", timestamp, method, path, string(body))
    sig := ed25519.Sign(privateKey, []byte(data))
    return base64.StdEncoding.EncodeToString(sig), nil
}

func VerifyRequest(publicKey ed25519.PublicKey, method, path string, timestamp int64, body []byte, sigStr string) (bool, error) {
    now := time.Now().Unix()
    if now-timestamp > 300 || timestamp-now > 300 {
        return false, fmt.Errorf("firma expirada o con marca de tiempo inválida")
    }
    data := fmt.Sprintf("%d:%s:%s:%s", timestamp, method, path, string(body))
    sig, err := base64.StdEncoding.DecodeString(sigStr)
    if err != nil {
        return false, fmt.Errorf("firma no es base64 válido: %w", err)
    }
    return ed25519.Verify(publicKey, []byte(data), sig), nil
}
```

**Librería:** `crypto/ed25519` (stdlib de Go) — firma digital asimétrica de alta seguridad.

#### API Key Interna
Las rutas internas requieren una API Key compartida:

**Archivo:** `internal/middleware/apikey.go`

```go
func InternalAPIKeyMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        apiKey := r.Header.Get("X-Internal-API-Key")
        expectedKey := os.Getenv("INTERNAL_API_KEY")
        if expectedKey == "" {
            expectedKey = "my_api_key"
        }
        if apiKey != expectedKey {
            http.Error(w, `{"error":"acceso interno no autorizado"}`, http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### 3.2 Validación de Consistencia

#### Consistencia en actualización de perfil
Se construye dinámicamente la consulta SQL para actualizar solo los campos provistos:

**Archivo:** `internal/user/repository.go`

```go
func (r *userRepository) Update(u *UsuarioEntity) error {
    query := "UPDATE usuarios SET updated_at = NOW()"
    args := []interface{}{}
    argCount := 0

    if u.Nombre != "" {
        argCount++
        query += fmt.Sprintf(", nombre = $%d", argCount)
        args = append(args, u.Nombre)
    }
    if u.Email != "" {
        argCount++
        query += fmt.Sprintf(", email = $%d", argCount)
        args = append(args, u.Email)
    }
    // Siempre actualiza teléfono (puede ser vacío para borrarlo)
    argCount++
    query += fmt.Sprintf(", telefono = NULLIF($%d, '')", argCount)
    args = append(args, u.Telefono)
    
    argCount++
    query += fmt.Sprintf(" WHERE id = $%d", argCount)
    args = append(args, u.ID)
    // ...
}
```

#### Consistencia en upsert de zonas
Se validan las relaciones entre usuario y zonas mediante `ON CONFLICT` y desactivación de zonas no enviadas:

**Archivo:** `internal/user/repository.go`

```go
func (r *userRepository) UpsertZonas(userID string, zonas []ZonaRequest) error {
    tx, err := r.db.Begin()
    // ...
    for _, zona := range zonas {
        _, err = tx.Exec(`
            INSERT INTO zonas_usuario (user_id, zona_nombre, latitud, longitud, radio_km, activo, fecha_actualizacion)
            VALUES ($1, $2, $3, $4, $5, true, NOW())
            ON CONFLICT (user_id, zona_nombre) 
            DO UPDATE SET latitud = EXCLUDED.latitud, ...`,
            userID, zona.ZonaNombre, zona.Latitud, zona.Longitud, radio)
        // ...
    }
    // Desactivar zonas que no vinieron
    _, err = tx.Exec("UPDATE zonas_usuario SET activo = false WHERE user_id = $1 AND zona_nombre NOT IN (...)", ...)
    return tx.Commit()
}
```

#### Consistencia en destinos recientes
Se mantiene un máximo de 10 destinos por usuario:

**Archivo:** `internal/user/repository.go`

```go
_, err = r.db.Exec(`
    DELETE FROM historial_destinos
    WHERE user_id = $1 AND id NOT IN (
        SELECT id FROM historial_destinos
        WHERE user_id = $1
        ORDER BY fecha_creacion DESC
        LIMIT 10
    )`,
    userID,
)
```

### 3.3 Validación de Integridad

#### Cifrado AES-256-GCM de datos sensibles
El número de teléfono se cifra antes de almacenar y se descifra al leer:

**Archivo:** `internal/user/entity.go`

```go
func (u *UsuarioEntity) BeforeSave(key []byte) error {
    if u.Telefono == "" {
        return nil
    }
    encrypted, err := security.Encrypt(u.Telefono, key)
    if err != nil {
        return err
    }
    u.Telefono = encrypted
    return nil
}

func (u *UsuarioEntity) AfterLoad(key []byte) error {
    if u.Telefono == "" {
        return nil
    }
    decrypted, err := security.Decrypt(u.Telefono, key)
    if err != nil {
        return err
    }
    u.Telefono = decrypted
    return nil
}
```

**Archivo:** `internal/security/crypto.go`

```go
func Encrypt(plaintext string, key []byte) (string, error) {
    block, err := aes.NewCipher(key)   // AES-256
    aesGCM, err := cipher.NewGCM(block) // Modo GCM (autenticado)
    nonce := make([]byte, aesGCM.NonceSize())
    io.ReadFull(rand.Reader, nonce)     // Nonce aleatorio
    ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}
```

**Librerías:** `crypto/aes`, `crypto/cipher` (stdlib de Go) — cifrado simétrico con autenticación integrada (AEAD).

#### Verificación de integridad en transmisión
Las peticiones internas usan firma Ed25519 que cubre método, ruta, timestamp y cuerpo, garantizando que los datos no fueron alterados durante la transmisión (ver [3.1](#firma-de-peticiones-internas-hmac-con-ed25519)).

### 3.4 Validación de Permisos

#### Middleware de Roles (RBAC)
Verifica que el usuario tenga el rol adecuado para acceder a rutas de administración:

**Archivo:** `internal/middleware/roles.go`

```go
func RoleMiddleware(pubKey ed25519.PublicKey, roles ...string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            tokenString := strings.TrimPrefix(authHeader, "Bearer ")

            claims, err := ValidateToken(tokenString, pubKey)
            if err != nil {
                http.Error(w, `{"error":"token inválido","code":401}`, http.StatusUnauthorized)
                return
            }

            tipo, ok := claims["tipo"].(string)
            if !ok {
                http.Error(w, `{"error":"sin permisos","code":403}`, http.StatusForbidden)
                return
            }

            for _, rol := range roles {
                if tipo == rol {
                    next.ServeHTTP(w, r)
                    return
                }
            }

            http.Error(w, `{"error":"acceso denegado: se requiere rol `+strings.Join(roles, " o ")+`","code":403}`, http.StatusForbidden)
        })
    }
}
```

#### Registro en el router (`cmd/api/main.go`)
```go
apiAdmin := api.PathPrefix("/admin").Subrouter()
apiAdmin.Use(middleware.RoleMiddleware(jwtPublicKey, "admin"))
apiAdmin.HandleFunc("/resumen", adminReporteHandler.GetAdminResumenHandler()).Methods("GET")
apiAdmin.HandleFunc("/registrar-conductor", authHandler.RegistrarConductorHandler()).Methods("POST")
// ...
```

---

## 4. Validación de Tipo

### Decodificación JSON con verificación de tipos
Se utiliza `json.Decoder` y `json.Unmarshal` de la stdlib de Go, que realizan validación de tipos automáticamente: si el JSON entrante no coincide con los tipos esperados en las structs, la decodificación falla.

**Ejemplo en todos los handlers:**

```go
var req LoginRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    common.WriteError(w, http.StatusBadRequest, "datos de entrada inválidos")
    return
}
```

**Librería:** `encoding/json` (stdlib de Go) — validación de tipos nativa: campos numéricos en structs `float64` rechazan strings, campos `string` rechazan números, etc.

### Validación de tipos primitivos en handlers

**Archivo:** `internal/reporte/handler.go` — Parseo explícito con verificación de errores:

```go
lat, err := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
if err != nil {
    common.WriteError(w, http.StatusBadRequest, "latitud inválida")
    return
}
lon, err := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
if err != nil {
    common.WriteError(w, http.StatusBadRequest, "longitud inválida")
    return
}
```

**Librería:** `strconv` (stdlib de Go) — conversión y validación de tipos numéricos.

### Validación de tipos en claims JWT
**Archivo:** `internal/middleware/auth.go`

```go
userID, ok := claims["user_id"].(string)
if !ok || userID == "" {
    http.Error(w, "User ID not found in token", http.StatusUnauthorized)
    return
}
```

---

## 5. Validación de Lógica de Negocio

### Límite de destinos recientes
Se mantiene un máximo de 10 destinos por usuario (ver [3.2](#consistencia-en-destinos-recientes)).

### Asignación automática de tipo de conductor
**Archivo:** `internal/auth/pipe.go`

```go
if req.Tipo == "" {
    req.Tipo = "conductor"  // Valor por defecto
}
```

### Valor por defecto de RutaID
**Archivo:** `internal/reporte/pipe.go`

```go
if strings.TrimSpace(req.RutaID) == "" {
    req.RutaID = "sin-ruta"
}
```

### Descripción automática según tipo de reporte
**Archivo:** `internal/reporte/pipe.go`

```go
func generarDescripcion(tipo string) string {
    descripciones := map[string]string{
        "accidente":  "Accidente vial reportado en la vía",
        "inundacion": "Inundación reportada, precaución al circular",
        // ...
    }
    if desc, ok := descripciones[tipo]; ok {
        return desc
    }
    return "Incidente vial reportado"
}
```

### Radio por defecto para zonas
**Archivo:** `internal/user/repository.go`

```go
radio := zona.RadioKm
if radio == 0 {
    radio = 15.0  // km
}
```

### Cálculo de total de páginas en paginación
**Archivo:** `internal/user/service.go`

```go
totalPaginas := (total + limit - 1) / limit
if totalPaginas == 0 {
    totalPaginas = 1
}
```

---

## 6. Validación de Patrones y Reglas Específicas

### 6.1 Direcciones de Correo Electrónico

**Validación actual:** Verificación básica de presencia de `@`.

**Archivo:** `internal/auth/pipe.go`
```go
if !strings.Contains(req.Email, "@") {
    return fmt.Errorf("el campo 'email' no tiene un formato válido")
}
```

**Mejora sugerida:** Implementar validación con regex:
```go
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
if !emailRegex.MatchString(req.Email) {
    return fmt.Errorf("formato de email inválido")
}
```

### 6.2 Números de Tarjeta de Crédito
No aplica actualmente en SafeRoute (no hay procesamiento de pagos).

### 6.3 Contraseñas

**Archivo:** `internal/auth/pipe.go`
```go
if len(req.Password) < 6 {
    return fmt.Errorf("la contraseña debe tener al menos 6 caracteres")
}
```

**Mejora sugerida:** Implementar política más robusta:
```go
var (
    mayuscula = regexp.MustCompile(`[A-Z]`)
    minuscula = regexp.MustCompile(`[a-z]`)
    numero    = regexp.MustCompile(`[0-9]`)
    especial  = regexp.MustCompile(`[!@#$%^&*(),.?":{}|<>]`)
)
if len(req.Password) < 8 {
    return fmt.Errorf("la contraseña debe tener al menos 8 caracteres")
}
if !mayuscula.MatchString(req.Password) {
    return fmt.Errorf("la contraseña debe contener al menos una mayúscula")
}
if !minuscula.MatchString(req.Password) {
    return fmt.Errorf("la contraseña debe contener al menos una minúscula")
}
if !numero.MatchString(req.Password) {
    return fmt.Errorf("la contraseña debe contener al menos un número")
}
```

### 6.4 Otros Patrones

#### Validación de tipos de incidente (whitelist)
**Archivo:** `internal/reporte/models.go` y `internal/reporte/pipe.go`

```go
var TiposValidos = map[string]bool{
    "accidente":  true,
    "inundacion": true,
    "bache":      true,
    "derrumbe":   true,
    "sin_luz":    true,
    "niebla":     true,
    "bloqueo":    true,
    "otro":       true,
}
```

#### Validación de tipo de usuario
**Archivo:** `internal/auth/pipe.go`
```go
if req.Tipo != "conductor" {
    return fmt.Errorf("el campo 'tipo' solo puede ser 'conductor' para el registro público")
}
```

---

## 7. Validación Cruzada

### Coordenadas de origen y destino en viajes
Se valida que ambas coordenadas (origen y destino) sean provistas:

**Archivo:** `internal/viaje/pipe.go`

```go
if req.OrigenLat == 0 && req.OrigenLon == 0 {
    return fmt.Errorf("las coordenadas de origen (origen_lat, origen_lon) son requeridas")
}
if req.DestinoLat == 0 && req.DestinoLon == 0 {
    return fmt.Errorf("las coordenadas de destino (destino_lat, destino_lon) son requeridas")
}
```

### Validación de coordenadas en motor de rutas
Se verifica que todas las coordenadas estén presentes antes de llamar al motor externo:

**Archivo:** `internal/motor/handler.go`

```go
if req.OrigenLat == 0 || req.OrigenLon == 0 || req.DestinoLat == 0 || req.DestinoLon == 0 {
    common.WriteError(w, http.StatusBadRequest, "todas las coordenadas son requeridas")
    return
}
```

### Validación de latitud/longitud en reportes cercanos
**Archivo:** `internal/reporte/handler.go`

```go
lat, err := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
if err != nil {
    common.WriteError(w, http.StatusBadRequest, "latitud inválida")
    return
}
lon, err := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
if err != nil {
    common.WriteError(w, http.StatusBadRequest, "longitud inválida")
    return
}
```

### Suscripción/Desuscripción de rutas
Se valida que `user_id` (del token) y `ruta_id` (del body/query) estén presentes y sean coherentes:

**Archivo:** `internal/user/suscripciones_handler.go`

```go
func (h *Handler) SuscribirRutaHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := middleware.GetUserID(r)
        if userID == "" {
            common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
            return
        }
        var req struct { RutaID string `json:"ruta_id"` }
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            common.WriteError(w, http.StatusBadRequest, "datos inválidos")
            return
        }
        if req.RutaID == "" {
            common.WriteError(w, http.StatusBadRequest, "ruta_id es requerido")
            return
        }
        // ...
    }
}
```

---

## 8. Validación Contextual

### Verificación de viaje activo del usuario

**Archivo:** `internal/viaje/handler.go`

```go
func (h *Handler) GetActiveViajeHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := middleware.GetUserID(r)
        if userID == "" {
            common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
            return
        }
        viaje, err := h.viajeSvc.GetActiveViaje(userID)
        // ...
        if viaje == nil {
            common.WriteError(w, http.StatusNotFound, "no tienes un viaje activo actualmente")
            return
        }
        // ...
    }
}
```

### Obtención de reportes cercanos por ubicación
Se valida que la ubicación del usuario esté dentro de un radio de búsqueda:

**Archivo:** `internal/reporte/handler.go`

```go
func (h *Handler) GetReportesCercanosHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        lat, err := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
        lon, err := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
        radioKm := 30.0
        if rv, err := strconv.ParseFloat(r.URL.Query().Get("radio_km"), 64); err == nil && rv > 0 && rv <= 100 {
            radioKm = rv
        }
        reportes, err := h.reporteSvc.GetCercanos(lat, lon, radioKm, 50)
        // ...
    }
}
```

### Validación de pertenencia de destino al usuario
**Archivo:** `internal/user/repository.go`

```go
func (r *userRepository) DeleteDestino(userID string, destinoID string) error {
    _, err := r.db.Exec(
        "DELETE FROM historial_destinos WHERE user_id = $1 AND id = $2",
        userID, destinoID,
    )
    return err
}
```

### Validación de pertenencia de notificación al usuario
**Archivo:** `internal/user/repository.go`

```go
func (r *userRepository) MarkNotification(userID string, notifID string, leida bool) error {
    _, err = r.db.Exec(
        `UPDATE notificaciones_historial SET leida = true, fecha_lectura = NOW() 
         WHERE id = $1 AND user_id = $2`,
        notifID, userID,
    )
    return err
}
```

---

## 9. Sanitización de Entrada

### 9a. Escapado de Caracteres

#### HTML Escaping
No se implementa escapado HTML manual porque la API devuelve JSON, no HTML. El motor de serialización `encoding/json` de Go escapa automáticamente caracteres especiales como `<`, `>`, `&` en strings JSON.

**Ejemplo implícito:**
```go
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(data)
// encoding/json escapa automáticamente caracteres HTML en strings
```

#### JavaScript Escaping
No aplica directamente (API REST). Las respuestas JSON son seguras por diseño contra inyección XSS cuando se establece `Content-Type: application/json`.

#### SQL Escaping
Se utiliza **consultas parametrizadas** con placeholders (`$1`, `$2`, etc.) para prevenir inyección SQL. El driver `lib/pq` se encarga del escapado automático:

**Ejemplo en todos los repositorios:**
```go
_, err := r.db.Exec(
    `INSERT INTO usuarios (email, password_hash, nombre, tipo, telefono)
     VALUES ($1, $2, $3, $4, $5)`,
    u.Email, u.PasswordHash, u.Nombre, u.Tipo, u.Telefono,
)
```

**Librerías:** `database/sql` + `github.com/lib/pq` — proporcionan consultas parametrizadas que separan el código SQL de los datos.

### 9b. Filtrado de Entradas

#### Whitelisting (Lista Blanca)

**Tipos de incidente permitidos:**
```go
var tiposPermitidosPipe = map[string]bool{
    "accidente":  true,
    "inundacion": true,
    // ... solo estos 8 tipos son aceptados
}
```

**Tipo de usuario permitido:**
```go
if req.Tipo != "conductor" {
    return fmt.Errorf("el campo 'tipo' solo puede ser 'conductor'")
}
```

#### Blacklisting (Lista Negra)
No se utiliza blacklisting actualmente. Se prefiere whitelisting por ser más seguro.

### 9c. Validación de Tipo de Datos

#### Tipos Primitivos
La decodificación JSON (`encoding/json`) valida tipos automáticamente. Por ejemplo, si se espera `float64` y se recibe un string, la decodificación falla.

```go
type RutasRequest struct {
    OrigenLat  float64 `json:"origen_lat"`  // Fallará si recibe un string
    OrigenLon  float64 `json:"origen_lon"`
    DestinoLat float64 `json:"destino_lat"`
    DestinoLon float64 `json:"destino_lon"`
}
```

#### Estructuras de Datos
Se valida que las estructuras JSON entrantes tengan el formato esperado. Cualquier campo extra es ignorado por `json.Unmarshal`.

### 9d. Limpieza de Entradas (Input Cleaning)

#### Trim
Se eliminan espacios en blanco al inicio y final de las cadenas en todos los puntos de entrada:

**Archivos:** `internal/auth/pipe.go`, `internal/reporte/pipe.go`, `internal/viaje/pipe.go`

```go
req.Email = strings.ToLower(strings.TrimSpace(req.Email))
req.Nombre = strings.TrimSpace(req.Nombre)
req.Telefono = strings.TrimSpace(req.Telefono)
req.RutaID = strings.TrimSpace(req.RutaID)
req.PolylineRuta = strings.TrimSpace(req.PolylineRuta)
```

**Librería:** `strings` (stdlib de Go).

#### Normalize
Se normalizan cadenas a minúsculas para evitar duplicados por capitalización:

```go
req.Email = strings.ToLower(strings.TrimSpace(req.Email))
req.Tipo = strings.ToLower(strings.TrimSpace(req.Tipo))
```

### 9e. Codificación de Entradas (Input Encoding)

#### Base64 Encoding
Se utiliza para codificar el ciphertext AES-256-GCM y las firmas Ed25519:

**Archivo:** `internal/security/crypto.go`
```go
return base64.StdEncoding.EncodeToString(ciphertext), nil
```

**Archivo:** `internal/security/signing.go`
```go
return base64.StdEncoding.EncodeToString(sig), nil
```

#### URL Encoding
No se implementa manualmente; Go lo maneja automáticamente en `net/http`.

### 9f. Uso de Funciones y Librerías Seguras

#### ORMs (Object-Relational Mappers)
No se utiliza ORM. Se usa `database/sql` directamente con consultas parametrizadas, que es igualmente seguro contra inyección SQL cuando se usan placeholders.

#### Librerías de Escapado
- `github.com/lib/pq` — escapado automático de parámetros SQL
- `golang.org/x/crypto` — funciones criptográficas seguras
- `encoding/json` — serialización segura (escapado de caracteres en JSON)
- `crypto/aes` + `crypto/cipher` (stdlib) — cifrado AES-256-GCM

### 9g. Reemplazo de Caracteres

#### Reemplazo de Comillas
No se realiza reemplazo manual de comillas. El uso de consultas parametrizadas (`$1`, `$2`) en SQL hace innecesario el escapado manual de comillas.

#### Reemplazo de Scripts
No aplica directamente (API REST con respuestas JSON). Los scripts incrustados no son ejecutables en respuestas JSON.

### 9h. Canonicalización

#### Path Normalization
No se implementa explícitamente. `gorilla/mux` maneja la normalización de rutas automáticamente.

#### Case Normalization
Se normalizan emails y tipos a minúsculas:

```go
req.Email = strings.ToLower(strings.TrimSpace(req.Email))
req.Tipo = strings.ToLower(strings.TrimSpace(req.Tipo))
```

### 9i. Escape Output Contextually

#### HTML Context
No aplica (API REST devuelve JSON, no HTML).

#### JavaScript Context
No aplica (API REST).

#### URL Context
No se realiza escapado manual; Go lo maneja automáticamente en `net/http`.

### 9j. Revisiones y Auditorías de Código

#### Código Estático
Se recomienda usar herramientas como:
- `go vet` — análisis estático incluido en Go
- `staticcheck` — linter avanzado para Go
- `gosec` — análisis de seguridad para Go

#### Pruebas de Penetración
Se recomienda realizar pruebas de penetración periódicas enfocadas en:
- Inyección SQL
- XSS (aunque la API es REST)
- Falsificación de tokens JWT
- Ataques de fuerza bruta (rate limiting implementado)
- Manipulación de firmas Ed25519
- Ataques de repetición (replay attacks)

---

## 10. Uso de Librerías y Frameworks de Validación

### Resumen de librerías utilizadas para validación:

| Librería | Propósito | Seguridad |
|---|---|---|
| `encoding/json` | Decodificación y validación de tipos JSON | Escapa HTML en strings automáticamente |
| `strconv` | Conversión segura de strings a tipos numéricos | Maneja errores de conversión |
| `database/sql` + `lib/pq` | Consultas parametrizadas SQL | Previene inyección SQL |
| `golang-jwt/jwt/v5` | Validación de tokens JWT con Ed25519 | Verifica firma, método y expiración |
| `golang.org/x/time/rate` | Rate limiting por IP | Previene ataques de fuerza bruta |
| `rs/cors` | Control de acceso CORS | Previene accesos cross-origin no autorizados |
| `crypto/ed25519` | Firma digital de peticiones internas | Verifica autenticidad e integridad |
| `crypto/aes` + `crypto/cipher` | Cifrado AES-256-GCM | Protege datos sensibles en reposo |
| `gorilla/mux` | Enrutamiento con variables de path | Previene path traversal |

### Buenas prácticas implementadas:
1. **Defensa en profundidad**: Validación tanto en handlers como en capa de servicio/repositorio
2. **Principio de mínimo privilegio**: Roles RBAC para administradores
3. **Validación en el servidor**: No se confía en la validación del cliente
4. **Consultas parametrizadas**: 100% de las consultas SQL usan placeholders
5. **Cifrado de datos sensibles**: Teléfono cifrado con AES-256-GCM
6. **Rate limiting**: Límite de peticiones por IP
7. **Headers de seguridad**: X-Content-Type-Options, X-Frame-Options, CSP, HSTS

---

## 11. Educación y Capacitación del Equipo

### Prácticas recomendadas para el equipo de desarrollo:

1. **Talleres de seguridad OWASP Top 10** - Enfocados en validación de entradas
2. **Code reviews obligatorios** - Toda validación debe ser revisada por pares
3. **Documentación de validaciones** - Mantener actualizado este documento
4. **Pruebas unitarias de validación** - Cada función de validación debe tener tests
5. **CI/CD con análisis de seguridad** - Integrar `gosec` y `staticcheck` en el pipeline

### Recursos sugeridos:
- OWASP Input Validation Cheat Sheet: https://cheatsheetseries.owasp.org/cheatsheets/Input_Validation_Cheat_Sheet.html
- OWASP SQL Injection Prevention: https://cheatsheetseries.owasp.org/cheatsheets/SQL_Injection_Prevention_Cheat_Sheet.html

---

## 12. Gestión de Errores Adecuada

### Manejo centralizado de errores
Se utiliza una función común para escribir errores de forma consistente:

**Archivo:** `internal/common/web.go`

```go
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    int    `json:"code"`
    Detalle string `json:"detalle,omitempty"`
}

func WriteError(w http.ResponseWriter, code int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(ErrorResponse{
        Error: message,
        Code:  code,
    })
}
```

### Principios aplicados:
1. **Mensajes genéricos**: No se revelan detalles de implementación (`"error creando el reporte"` en lugar de detalles de la BD)
2. **Códigos de estado HTTP apropiados**: `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`, `404 Not Found`, `500 Internal Server Error`
3. **Logs detallados internamente**: La información sensible se registra en logs del servidor, no en respuestas al cliente
4. **Manejo de errores de BD**: Los errores de base de datos se traducen a mensajes genéricos

### Ejemplos de gestión de errores:

**Archivo:** `internal/reporte/handler.go`
```go
reporte, err := h.reporteSvc.Create(req, userID)
if err != nil {
    log.Printf("[REPORTE] Error creando: %v", err)                     // Log detallado interno
    common.WriteError(w, http.StatusInternalServerError, "error creando el reporte")  // Mensaje genérico
    return
}
```

**Archivo:** `internal/viaje/handler.go`
```go
viaje, err := h.viajeSvc.GetActiveViaje(userID)
if err != nil {
    log.Printf("[VIAJES] Error consultando viaje activo de %s: %v", userID, err)
    common.WriteError(w, http.StatusNotFound, "no se encontró un viaje activo")
    return
}
```

### Errores que NO se exponen al cliente:
- Detalles de errores de base de datos (columnas, tablas, constraints)
- Stack traces de errores
- Información de configuración interna
- Mensajes de error de librerías externas
- Detalles de claves criptográficas

---

## Checklist de Validaciones — SafeRoute API (Lado Servidor)

| # | Tipo de Validación | Estado | ¿Cómo/Dónde? | ¿Por qué no aplica? |
|---|---|---|---|---|
| 1 | Validación del lado del servidor | ✅ Implementado | `internal/auth/pipe.go`, `internal/reporte/pipe.go`, `internal/viaje/pipe.go` | — |
| 1a | Validación de formato (email) | ✅ Implementado | `strings.Contains(email, "@")` en `auth/pipe.go` | — |
| 1b | Validación de longitud (password) | ✅ Implementado | `len(password) < 6` en `auth/pipe.go` | — |
| 1c | Validación de rango (coordenadas) | ✅ Implementado | Rangos -90/90 y -180/180 en `reporte/pipe.go` y `viaje/pipe.go` | — |
| 1d | Validación de contenido (campos requeridos) | ✅ Implementado | Verificación `campo == ""` en todos los pipes | — |
| 1e | Validación regex | ❌ No implementado | No se usan regex actualmente (ver sugerencias en sección 6) | — |
| 2 | Validación lado servidor | ✅ Implementado | — | — |
| 2a | Validación de autenticidad (JWT) | ✅ Implementado | `middleware/auth.go` con `golang-jwt/jwt/v5` | — |
| 2b | Validación de consistencia | ✅ Implementado | Consultas con `WHERE user_id = $1` y transacciones | — |
| 2c | Validación de integridad | ✅ Implementado | Firma Ed25519 + cifrado AES-256-GCM | — |
| 2d | Validación de permisos (RBAC) | ✅ Implementado | `middleware/roles.go` con verificación de claims | — |
| 3 | Validación de tipo | ✅ Implementado | `encoding/json` + `strconv` de la stdlib | — |
| 4 | Validación de lógica de negocio | ✅ Implementado | Límite 10 destinos, radio 15km default, tipo conductor default | — |
| 5 | Validación de patrones específicos | Parcial | Email básico, password mínima, tipos whitelist | — |
| 5a | Email (formato completo) | ❌ Parcial | Solo verifica `@` — mejorar con regex | — |
| 5b | Tarjeta de crédito (Luhn) | ❌ No aplica | No hay procesamiento de pagos | — |
| 5c | Contraseñas (fortaleza) | ❌ Parcial | Solo longitud mínima 6 — mejorar | — |
| 6 | Validación cruzada | ✅ Implementado | Coordenadas origen/destino en viajes | — |
| 7 | Validación contextual | ✅ Implementado | Pertenencia de recursos al usuario autenticado | — |
| 8 | Sanitización de entrada | ✅ Implementado | — | — |
| 8a | Escapado (HTML/JS/SQL) | ✅ Parcial | SQL parametrizado; HTML escapado por JSON encoder | — |
| 8b | Filtrado (whitelist/blacklist) | ✅ Implementado | Whitelist de tipos de incidente y roles | — |
| 8c | Validación tipo datos | ✅ Implementado | JSON decoder nativo de Go | — |
| 8d | Limpieza (trim/normalize) | ✅ Implementado | `strings.TrimSpace` + `strings.ToLower` | — |
| 8e | Codificación (Base64/URL) | ✅ Implementado | Base64 para cifrado y firmas | — |
| 8f | Librerías seguras | ✅ Implementado | `lib/pq`, `crypto/ed25519`, `golang-jwt` | — |
| 8g | Reemplazo caracteres | ✅ No necesario | Consultas parametrizadas hacen innecesario el reemplazo | — |
| 8h | Canonicalización | ✅ Parcial | Case normalization de email y tipo | — |
| 8i | Escape contextual | ✅ No aplica | API REST con respuestas JSON | — |
| 8j | Auditorías de código | ✅ Recomendado | `gosec`, `staticcheck`, pruebas de penetración | — |
| 9 | Uso de librerías de validación | ✅ Implementado | Stdlib + `golang-jwt/jwt/v5` + `golang.org/x/time/rate` | — |
| 10 | Educación del equipo | ✅ Recomendado | Talleres OWASP, code reviews, documentación | — |
| 11 | Gestión de errores | ✅ Implementado | Mensajes genéricos, logs detallados internos | — |

---

> **Documento generado:** Julio 2026
> **Proyecto:** SafeRoute API v2.0
> **Lenguaje:** Go 1.25
> **Este documento se integra al reporte de validación de entradas de datos de SafeRoute, complementando la documentación del lado del cliente (Flutter) ya realizada.**