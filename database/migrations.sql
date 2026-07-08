-- 1. Habilitar la extensión de PostGIS (requiere permisos de superusuario en PostgreSQL)
-- En Neon, esta extensión se puede habilitar si no existe
CREATE EXTENSION IF NOT EXISTS postgis;

-- 2. Tabla para almacenar usuarios
CREATE TABLE IF NOT EXISTS usuarios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    nombre VARCHAR(255) NOT NULL,
    tipo VARCHAR(50) DEFAULT 'conductor',
    telefono VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    ultimo_acceso TIMESTAMP WITH TIME ZONE
);

-- 3. Tabla para almacenar reportes de incidentes
CREATE TABLE IF NOT EXISTS reportes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES usuarios(id) ON DELETE SET NULL,
    tipo VARCHAR(50) NOT NULL,
    latitud DOUBLE PRECISION NOT NULL,
    longitud DOUBLE PRECISION NOT NULL,
    nota_voz TEXT,
    ruta_id VARCHAR(255),
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    vigente BOOLEAN DEFAULT TRUE,
    confirmaciones INTEGER DEFAULT 0
);

-- 4. Tabla para suscripciones de usuarios a alertas de rutas específicas
CREATE TABLE IF NOT EXISTS suscripciones_rutas (
    user_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    ruta_id VARCHAR(255) NOT NULL,
    suscrito BOOLEAN DEFAULT TRUE,
    fecha_suscripcion TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    fecha_actualizacion TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, ruta_id)
);

-- 5. Tabla para las zonas de interés geográfico de los usuarios
CREATE TABLE IF NOT EXISTS zonas_usuario (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    zona_nombre VARCHAR(255) NOT NULL,
    latitud DOUBLE PRECISION NOT NULL,
    longitud DOUBLE PRECISION NOT NULL,
    radio_km DOUBLE PRECISION NOT NULL DEFAULT 15.0,
    activo BOOLEAN DEFAULT TRUE,
    fecha_creacion TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    fecha_actualizacion TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, zona_nombre)
);

-- 6. Tabla para almacenar viajes de conductores
CREATE TABLE IF NOT EXISTS viajes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    ruta_id VARCHAR(255) NOT NULL,
    origen_lat DOUBLE PRECISION NOT NULL,
    origen_lon DOUBLE PRECISION NOT NULL,
    destino_lat DOUBLE PRECISION NOT NULL,
    destino_lon DOUBLE PRECISION NOT NULL,
    polyline_ruta TEXT NOT NULL,
    geom_ruta GEOMETRY(LineString, 4326),
    estado VARCHAR(50) DEFAULT 'activo',
    fecha_inicio TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    fecha_fin TIMESTAMP WITH TIME ZONE,
    ultimo_heartbeat TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_estado CHECK (estado IN ('activo', 'finalizado', 'desviado', 'parada_tecnica', 'contacto_perdido', 'cancelado'))
);

-- 7. Tabla para almacenar el historial de coordenadas de telemetría por viaje
CREATE TABLE IF NOT EXISTS historial_viaje_coordenadas (
    id BIGSERIAL PRIMARY KEY,
    viaje_id UUID NOT NULL REFERENCES viajes(id) ON DELETE CASCADE,
    latitud DOUBLE PRECISION NOT NULL,
    longitud DOUBLE PRECISION NOT NULL,
    velocidad_kmh DOUBLE PRECISION,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 8. Tabla para el historial de notificaciones enviadas a usuarios
CREATE TABLE IF NOT EXISTS notificaciones_historial (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    tipo VARCHAR(50) NOT NULL,
    reporte_id UUID REFERENCES reportes(id) ON DELETE SET NULL,
    latitud DOUBLE PRECISION,
    longitud DOUBLE PRECISION,
    nota_voz TEXT,
    ruta_id VARCHAR(255),
    mensaje TEXT NOT NULL,
    leida BOOLEAN DEFAULT FALSE,
    fecha_envio TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    fecha_lectura TIMESTAMP WITH TIME ZONE
);

-- 9. Crear índices para búsquedas eficientes
CREATE INDEX IF NOT EXISTS idx_viajes_estado ON viajes(estado);
CREATE INDEX IF NOT EXISTS idx_viajes_user_id ON viajes(user_id);
CREATE INDEX IF NOT EXISTS idx_historial_viaje_id ON historial_viaje_coordenadas(viaje_id);
CREATE INDEX IF NOT EXISTS idx_viajes_geom ON viajes USING gist(geom_ruta);
CREATE INDEX IF NOT EXISTS idx_reportes_vigente ON reportes(vigente);
CREATE INDEX IF NOT EXISTS idx_notificaciones_usuario_leida ON notificaciones_historial(user_id, leida);
