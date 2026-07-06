-- 1. Habilitar la extensión de PostGIS (requiere permisos de superusuario en PostgreSQL)
-- En Neon, esta extensión se puede habilitar si no existe
CREATE EXTENSION IF NOT EXISTS postgis;

-- 2. Tabla para almacenar viajes
CREATE TABLE IF NOT EXISTS viajes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL, -- Eliminamos REFERENCES usuarios(id) temporalmente para simplificar pruebas o si los usuarios no son UUIDs
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

-- 3. Tabla para almacenar el historial de coordenadas de telemetría por viaje
CREATE TABLE IF NOT EXISTS historial_viaje_coordenadas (
    id BIGSERIAL PRIMARY KEY,
    viaje_id UUID NOT NULL REFERENCES viajes(id) ON DELETE CASCADE,
    latitud DOUBLE PRECISION NOT NULL,
    longitud DOUBLE PRECISION NOT NULL,
    velocidad_kmh DOUBLE PRECISION,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 4. Crear índices para búsquedas eficientes
CREATE INDEX IF NOT EXISTS idx_viajes_estado ON viajes(estado);
CREATE INDEX IF NOT EXISTS idx_viajes_user_id ON viajes(user_id);
CREATE INDEX IF NOT EXISTS idx_historial_viaje_id ON historial_viaje_coordenadas(viaje_id);
CREATE INDEX IF NOT EXISTS idx_viajes_geom ON viajes USING gist(geom_ruta);
