-- ============================================================
-- MIGRACIÓN: Módulo de Facturación (Billing / Planes Empresariales)
-- SafeRoute API v2.0
-- ============================================================

-- 1. Tabla de empresas (vinculada a admin)
CREATE TABLE IF NOT EXISTS empresas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    nombre_empresa VARCHAR(255) NOT NULL,
    rfc VARCHAR(13) DEFAULT '',
    email_facturacion VARCHAR(255) DEFAULT '',

    -- Plan y Stripe
    plan_actual VARCHAR(20) NOT NULL DEFAULT 'basico',
    stripe_customer_id VARCHAR(100) DEFAULT '',
    stripe_subscription_id VARCHAR(100) DEFAULT '',

    -- Estado de suscripción
    estado_suscripcion VARCHAR(20) NOT NULL DEFAULT 'pendiente',
    trial_ends_at TIMESTAMP WITH TIME ZONE,
    current_period_start TIMESTAMP WITH TIME ZONE,
    current_period_end TIMESTAMP WITH TIME ZONE,

    -- Conductores
    max_conductores INTEGER NOT NULL DEFAULT 15,
    conductores_extra INTEGER NOT NULL DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_estado_suscripcion CHECK (
        estado_suscripcion IN ('trial', 'activo', 'pendiente', 'cancelado', 'expirado')
    ),
    CONSTRAINT chk_plan CHECK (
        plan_actual IN ('basico', 'profesional')
    ),
    CONSTRAINT unique_admin_empresa UNIQUE (admin_id)
);

-- 2. Tabla de facturas
CREATE TABLE IF NOT EXISTS facturas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id UUID NOT NULL REFERENCES empresas(id) ON DELETE CASCADE,
    stripe_invoice_id VARCHAR(100) DEFAULT '',
    stripe_payment_intent_id VARCHAR(100) DEFAULT '',

    -- Montos
    subtotal DECIMAL(10,2) NOT NULL,
    iva DECIMAL(10,2) NOT NULL DEFAULT 0,
    total DECIMAL(10,2) NOT NULL,

    -- Detalle
    plan VARCHAR(20) NOT NULL,
    conductores_base INTEGER NOT NULL DEFAULT 0,
    conductores_extra INTEGER NOT NULL DEFAULT 0,
    cargo_conductores_extra DECIMAL(10,2) NOT NULL DEFAULT 0,
    periodo_inicio TIMESTAMP WITH TIME ZONE,
    periodo_fin TIMESTAMP WITH TIME ZONE,

    -- Estado
    estado VARCHAR(20) NOT NULL DEFAULT 'pendiente',
    metodo_pago VARCHAR(20) DEFAULT '',

    -- Fechas
    fecha_emision TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    fecha_pago TIMESTAMP WITH TIME ZONE,
    fecha_vencimiento TIMESTAMP WITH TIME ZONE,

    -- CFDI (para facturación mexicana)
    cfdi_uuid VARCHAR(50) DEFAULT '',
    cfdi_xml TEXT DEFAULT '',
    cfdi_pdf_url VARCHAR(500) DEFAULT '',

    CONSTRAINT chk_estado_factura CHECK (
        estado IN ('pendiente', 'pagado', 'cancelado', 'reembolsado', 'vencido')
    )
);

-- 3. Tabla de transacciones (pagos individuales)
CREATE TABLE IF NOT EXISTS pagos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    factura_id UUID NOT NULL REFERENCES facturas(id) ON DELETE CASCADE,
    empresa_id UUID NOT NULL REFERENCES empresas(id) ON DELETE CASCADE,

    -- Proveedor de pago
    proveedor VARCHAR(30) NOT NULL,
    provider_payment_id VARCHAR(255) DEFAULT '',
    provider_checkout_id VARCHAR(255) DEFAULT '',

    -- Monto
    monto DECIMAL(10,2) NOT NULL,
    moneda VARCHAR(3) NOT NULL DEFAULT 'MXN',

    -- Estado
    estado VARCHAR(20) NOT NULL DEFAULT 'pendiente',
    metadata JSONB DEFAULT '{}',

    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_estado_pago CHECK (
        estado IN ('pendiente', 'completado', 'fallido', 'reembolsado')
    ),
    CONSTRAINT chk_proveedor CHECK (
        proveedor IN ('stripe', 'mercadopago', 'paypal')
    )
);

-- 4. Tabla de historial de cambios en suscripciones
CREATE TABLE IF NOT EXISTS historial_suscripcion (
    id BIGSERIAL PRIMARY KEY,
    empresa_id UUID NOT NULL REFERENCES empresas(id) ON DELETE CASCADE,
    cambio VARCHAR(50) NOT NULL,
    descripcion TEXT DEFAULT '',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_cambio CHECK (
        cambio IN (
            'creacion', 'activacion', 'cambio_plan', 'agregar_conductor',
            'remover_conductor', 'pago_recibido', 'pago_fallido',
            'cancelacion', 'renovacion', 'expiracion', 'factura_generada'
        )
    )
);

-- 5. Agregar empresa_id a usuarios (después de crear empresas)
-- Sin foreign key para mayor flexibilidad (Auth Service puede tener BD separada)
ALTER TABLE usuarios ADD COLUMN IF NOT EXISTS empresa_id UUID;

-- 6. Índices
CREATE INDEX IF NOT EXISTS idx_usuarios_empresa ON usuarios(empresa_id);
CREATE INDEX IF NOT EXISTS idx_empresas_admin_id ON empresas(admin_id);
CREATE INDEX IF NOT EXISTS idx_empresas_stripe_customer ON empresas(stripe_customer_id);
CREATE INDEX IF NOT EXISTS idx_empresas_stripe_subscription ON empresas(stripe_subscription_id);
CREATE INDEX IF NOT EXISTS idx_empresas_estado ON empresas(estado_suscripcion);
CREATE INDEX IF NOT EXISTS idx_facturas_empresa_id ON facturas(empresa_id);
CREATE INDEX IF NOT EXISTS idx_facturas_estado ON facturas(estado);
CREATE INDEX IF NOT EXISTS idx_facturas_stripe_invoice ON facturas(stripe_invoice_id);
CREATE INDEX IF NOT EXISTS idx_pagos_factura_id ON pagos(factura_id);
CREATE INDEX IF NOT EXISTS idx_pagos_provider_id ON pagos(provider_payment_id);
CREATE INDEX IF NOT EXISTS idx_historial_empresa_id ON historial_suscripcion(empresa_id);
CREATE INDEX IF NOT EXISTS idx_historial_fecha ON historial_suscripcion(created_at);