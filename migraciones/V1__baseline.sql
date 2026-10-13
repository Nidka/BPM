-- =============================================================================
-- BASE DE DATOS BPM - MIGRACIÓN V1
-- Línea base PostgreSQL para una instalación nueva
-- =============================================================================
-- Ejecutar sobre una base vacía.
-- Los archivos multimedia se almacenan fuera de PostgreSQL; aquí solo se guardan
-- su clave privada, URI y metadatos.
-- Los teléfonos se guardan como texto para admitir prefijo internacional y ceros.
-- =============================================================================

BEGIN;

-- ============================================================================
-- 1. TENANCY, IDENTIDAD Y AUDITORÍA
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Empresas o tenants, con identificación fiscal y contacto administrativo básico.
CREATE TABLE organizations (
    id VARCHAR(64) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    legal_name VARCHAR(200) NOT NULL,
    trade_name VARCHAR(200),
    tax_id VARCHAR(20),
    business_type VARCHAR(32) NOT NULL,
    contact_name VARCHAR(160),
    contact_phone VARCHAR(30),
    contact_email VARCHAR(320),
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    CONSTRAINT organizations_business_type_check
        CHECK (business_type IN ('RESTAURANT', 'CAFETERIA', 'CATERING', 'RELATED_SERVICE')),
    CONSTRAINT organizations_status_check CHECK (status IN ('ACTIVE', 'INACTIVE'))
);

-- Sedes físicas de la organización, incluyendo dirección y datos de contacto.
CREATE TABLE locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(id),
    name VARCHAR(160) NOT NULL,
    address VARCHAR(300) NOT NULL,
    address_reference VARCHAR(240),
    district VARCHAR(120),
    province VARCHAR(120),
    department VARCHAR(120),
    contact_phone VARCHAR(30),
    contact_email VARCHAR(320),
    timezone VARCHAR(80) NOT NULL DEFAULT 'America/Lima',
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, name),
    CONSTRAINT locations_status_check CHECK (status IN ('ACTIVE', 'INACTIVE'))
);

-- Identidades internas de usuarios vinculables al UID del proveedor de autenticación.
CREATE TABLE application_users (
    id VARCHAR(128) PRIMARY KEY,
    email VARCHAR(320) NOT NULL,
    display_name VARCHAR(200) NOT NULL,
    phone_number VARCHAR(30),
    platform_role VARCHAR(20) NOT NULL DEFAULT 'USER',
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    CONSTRAINT application_users_platform_role_check
        CHECK (platform_role IN ('USER', 'SUPER_ADMIN')),
    CONSTRAINT application_users_status_check CHECK (status IN ('ACTIVE', 'INACTIVE'))
);

CREATE UNIQUE INDEX application_users_email_uq ON application_users (LOWER(email));

-- Relación de un usuario con una organización, incluyendo rol y estado dentro del tenant.
CREATE TABLE organization_memberships (
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(id),
    user_id VARCHAR(128) NOT NULL REFERENCES application_users(id),
    role VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    PRIMARY KEY (organization_id, user_id),
    CONSTRAINT organization_memberships_role_check CHECK (role IN ('ADMIN', 'CHEF', 'WORKER')),
    CONSTRAINT organization_memberships_status_check CHECK (status IN ('ACTIVE', 'INACTIVE'))
);

-- Sedes específicas a las que tiene acceso un miembro de una organización.
CREATE TABLE membership_locations (
    organization_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    location_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    PRIMARY KEY (organization_id, user_id, location_id),
    FOREIGN KEY (organization_id, user_id)
        REFERENCES organization_memberships(organization_id, user_id),
    FOREIGN KEY (organization_id, location_id)
        REFERENCES locations(organization_id, id)
);

-- Instalaciones móviles registradas para sincronización y notificaciones push.
CREATE TABLE mobile_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    installation_id VARCHAR(128) NOT NULL,
    platform VARCHAR(20) NOT NULL,
    push_token TEXT,
    last_seen_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, installation_id),
    FOREIGN KEY (organization_id, user_id)
        REFERENCES organization_memberships(organization_id, user_id),
    CONSTRAINT mobile_devices_platform_check CHECK (platform IN ('ANDROID', 'IOS', 'WEB')),
    CONSTRAINT mobile_devices_status_check CHECK (status IN ('ACTIVE', 'INACTIVE'))
);

-- Resultados de solicitudes repetibles para evitar duplicados durante reintentos u operación offline.
CREATE TABLE idempotency_records (
    id BIGSERIAL PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(id),
    idempotency_key VARCHAR(160) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    response_status INTEGER,
    response_body JSONB,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, idempotency_key)
);

-- Bitácora de acciones sensibles realizadas sobre entidades del sistema.
CREATE TABLE audit_events (
    id BIGSERIAL PRIMARY KEY,
    organization_id VARCHAR(64) REFERENCES organizations(id),
    actor_user_id VARCHAR(128),
    action VARCHAR(120) NOT NULL,
    entity_type VARCHAR(100) NOT NULL,
    entity_id VARCHAR(128),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128)
);

-- Cola transaccional de eventos pendientes para notificaciones, correos e integraciones externas.
CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) REFERENCES organizations(id),
    event_type VARCHAR(120) NOT NULL,
    aggregate_type VARCHAR(100) NOT NULL,
    aggregate_id VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    CONSTRAINT outbox_events_status_check CHECK (status IN ('PENDING', 'PROCESSING', 'PROCESSED', 'FAILED')),
    CONSTRAINT outbox_events_attempts_check CHECK (attempts >= 0)
);

CREATE UNIQUE INDEX organizations_tax_id_uq ON organizations (tax_id) WHERE tax_id IS NOT NULL;
CREATE INDEX locations_organization_status_idx ON locations (organization_id, status);
CREATE INDEX locations_district_idx ON locations (organization_id, district, status);
CREATE INDEX memberships_user_status_idx ON organization_memberships (user_id, status);
CREATE INDEX membership_locations_location_idx ON membership_locations (organization_id, location_id);
CREATE INDEX mobile_devices_user_status_idx ON mobile_devices (organization_id, user_id, status);
CREATE INDEX idempotency_records_expiry_idx ON idempotency_records (expires_at);
CREATE INDEX audit_events_tenant_time_idx ON audit_events (organization_id, occurred_at DESC);
CREATE INDEX audit_events_entity_idx ON audit_events (organization_id, entity_type, entity_id);
CREATE INDEX outbox_events_pending_idx ON outbox_events (status, available_at) WHERE status IN ('PENDING', 'FAILED');

CREATE TRIGGER organizations_set_updated_at BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER locations_set_updated_at BEFORE UPDATE ON locations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER application_users_set_updated_at BEFORE UPDATE ON application_users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER organization_memberships_set_updated_at BEFORE UPDATE ON organization_memberships
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER membership_locations_set_updated_at BEFORE UPDATE ON membership_locations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER mobile_devices_set_updated_at BEFORE UPDATE ON mobile_devices
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER idempotency_records_set_updated_at BEFORE UPDATE ON idempotency_records
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER outbox_events_set_updated_at BEFORE UPDATE ON outbox_events
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================================
-- 2. CHECKLISTS Y ARCHIVOS MULTIMEDIA
-- ============================================================================

-- Metadatos de archivos privados almacenados externamente en Cloudflare R2; no contiene binarios.
CREATE TABLE media_files (
    id UUID PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(id),
    location_id UUID,
    object_key VARCHAR(500) NOT NULL,
    storage_url TEXT NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    sha256 VARCHAR(64),
    status VARCHAR(24) NOT NULL DEFAULT 'PENDING_UPLOAD',
    uploaded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (object_key),
    FOREIGN KEY (organization_id, location_id)
        REFERENCES locations(organization_id, id),
    CONSTRAINT media_files_size_check CHECK (size_bytes > 0),
    CONSTRAINT media_files_status_check
        CHECK (status IN ('PENDING_UPLOAD', 'AVAILABLE', 'FAILED', 'DELETED')),
    CONSTRAINT media_files_mime_check
        CHECK (mime_type LIKE 'image/%' OR mime_type LIKE 'video/%' OR mime_type = 'application/pdf')
);

-- Definiciones generales, descripción y área de los checklists BPM.
CREATE TABLE checklist_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(id),
    name VARCHAR(180) NOT NULL,
    description VARCHAR(500),
    area VARCHAR(80) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, name),
    CONSTRAINT checklist_templates_status_check CHECK (status IN ('ACTIVE', 'INACTIVE'))
);

-- Versiones publicadas o en borrador que congelan la estructura histórica de un checklist.
CREATE TABLE checklist_template_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    template_id UUID NOT NULL,
    version_number INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, template_id, version_number),
    FOREIGN KEY (organization_id, template_id)
        REFERENCES checklist_templates(organization_id, id),
    CONSTRAINT checklist_template_versions_number_check CHECK (version_number > 0),
    CONSTRAINT checklist_template_versions_status_check
        CHECK (status IN ('DRAFT', 'PUBLISHED', 'ARCHIVED'))
);

-- Preguntas, secciones y criterios de evaluación pertenecientes a una versión de checklist.
CREATE TABLE checklist_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    template_version_id UUID NOT NULL,
    section_name VARCHAR(120) NOT NULL,
    label VARCHAR(300) NOT NULL,
    response_type VARCHAR(24) NOT NULL,
    display_order INTEGER NOT NULL,
    required BOOLEAN NOT NULL DEFAULT TRUE,
    minimum_value NUMERIC(10, 2),
    maximum_value NUMERIC(10, 2),
    unit VARCHAR(20),
    evidence_required_on_failure BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (template_version_id, display_order),
    FOREIGN KEY (organization_id, template_version_id)
        REFERENCES checklist_template_versions(organization_id, id),
    CONSTRAINT checklist_items_response_type_check
        CHECK (response_type IN ('COMPLIANCE', 'TEMPERATURE', 'TEXT', 'NUMBER')),
    CONSTRAINT checklist_items_order_check CHECK (display_order > 0),
    CONSTRAINT checklist_items_range_check
        CHECK (minimum_value IS NULL OR maximum_value IS NULL OR minimum_value <= maximum_value)
);

-- Programación de checklists por sede, rol, turno y frecuencia.
CREATE TABLE checklist_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    location_id UUID NOT NULL,
    template_id UUID NOT NULL,
    frequency VARCHAR(20) NOT NULL,
    due_time TIME,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, location_id)
        REFERENCES locations(organization_id, id),
    FOREIGN KEY (organization_id, template_id)
        REFERENCES checklist_templates(organization_id, id),
    CONSTRAINT checklist_assignments_frequency_check
        CHECK (frequency IN ('ONCE', 'DAILY', 'WEEKLY', 'MONTHLY'))
);

-- Ejecuciones o borradores de checklist realizados en una sede, incluidos los creados offline.
CREATE TABLE checklist_runs (
    id UUID PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL,
    location_id UUID NOT NULL,
    template_version_id UUID NOT NULL,
    assignment_id UUID,
    performed_by VARCHAR(128) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    compliance_percent NUMERIC(5, 2),
    client_operation_id UUID NOT NULL,
    client_created_at TIMESTAMPTZ NOT NULL,
    submitted_at TIMESTAMPTZ,
    row_version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, client_operation_id),
    FOREIGN KEY (organization_id, location_id)
        REFERENCES locations(organization_id, id),
    FOREIGN KEY (organization_id, template_version_id)
        REFERENCES checklist_template_versions(organization_id, id),
    FOREIGN KEY (organization_id, assignment_id)
        REFERENCES checklist_assignments(organization_id, id),
    FOREIGN KEY (organization_id, performed_by)
        REFERENCES organization_memberships(organization_id, user_id),
    CONSTRAINT checklist_runs_status_check CHECK (status IN ('DRAFT', 'SUBMITTED', 'CANCELLED')),
    CONSTRAINT checklist_runs_compliance_check
        CHECK (compliance_percent IS NULL OR compliance_percent BETWEEN 0 AND 100),
    CONSTRAINT checklist_runs_version_check CHECK (row_version > 0)
);

-- Respuestas y resultados registrados para cada ítem de una ejecución de checklist.
CREATE TABLE checklist_answers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    checklist_run_id UUID NOT NULL,
    checklist_item_id UUID NOT NULL,
    compliance_result VARCHAR(24),
    numeric_value NUMERIC(10, 2),
    text_value TEXT,
    observation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (checklist_run_id, checklist_item_id),
    FOREIGN KEY (organization_id, checklist_run_id)
        REFERENCES checklist_runs(organization_id, id),
    FOREIGN KEY (organization_id, checklist_item_id)
        REFERENCES checklist_items(organization_id, id),
    CONSTRAINT checklist_answers_result_check
        CHECK (compliance_result IS NULL OR compliance_result IN ('COMPLIANT', 'NON_COMPLIANT', 'NOT_APPLICABLE'))
);

-- Relación entre evidencias multimedia y una ejecución o respuesta de checklist.
CREATE TABLE checklist_media (
    organization_id VARCHAR(64) NOT NULL,
    checklist_run_id UUID NOT NULL,
    checklist_answer_id UUID,
    media_file_id UUID NOT NULL,
    purpose VARCHAR(24) NOT NULL DEFAULT 'EVIDENCE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    PRIMARY KEY (organization_id, checklist_run_id, media_file_id),
    FOREIGN KEY (organization_id, checklist_run_id)
        REFERENCES checklist_runs(organization_id, id),
    FOREIGN KEY (organization_id, checklist_answer_id)
        REFERENCES checklist_answers(organization_id, id),
    FOREIGN KEY (organization_id, media_file_id)
        REFERENCES media_files(organization_id, id),
    CONSTRAINT checklist_media_purpose_check CHECK (purpose IN ('EVIDENCE', 'CORRECTION'))
);

CREATE INDEX media_files_tenant_status_idx ON media_files (organization_id, status, created_at DESC);
CREATE INDEX media_files_location_idx ON media_files (organization_id, location_id, created_at DESC);
CREATE INDEX checklist_templates_tenant_status_idx ON checklist_templates (organization_id, status);
CREATE INDEX checklist_versions_template_idx ON checklist_template_versions (organization_id, template_id, version_number DESC);
CREATE INDEX checklist_items_version_order_idx ON checklist_items (template_version_id, display_order);
CREATE INDEX checklist_assignments_location_idx ON checklist_assignments (organization_id, location_id, active);
CREATE INDEX checklist_runs_location_date_idx ON checklist_runs (organization_id, location_id, client_created_at DESC);
CREATE INDEX checklist_runs_status_idx ON checklist_runs (organization_id, status, updated_at DESC);
CREATE INDEX checklist_runs_user_idx ON checklist_runs (organization_id, performed_by, client_created_at DESC);

CREATE TRIGGER media_files_set_updated_at BEFORE UPDATE ON media_files
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER checklist_templates_set_updated_at BEFORE UPDATE ON checklist_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER checklist_template_versions_set_updated_at BEFORE UPDATE ON checklist_template_versions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER checklist_items_set_updated_at BEFORE UPDATE ON checklist_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER checklist_assignments_set_updated_at BEFORE UPDATE ON checklist_assignments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER checklist_runs_set_updated_at BEFORE UPDATE ON checklist_runs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER checklist_answers_set_updated_at BEFORE UPDATE ON checklist_answers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER checklist_media_set_updated_at BEFORE UPDATE ON checklist_media
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================================
-- 3. OPERACIÓN DIARIA
-- ============================================================================

-- Proveedores con identificación fiscal, dirección y contacto comercial básico.
CREATE TABLE suppliers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(id),
    name VARCHAR(200) NOT NULL,
    tax_id VARCHAR(20),
    contact_name VARCHAR(160),
    contact_phone VARCHAR(30),
    contact_email VARCHAR(320),
    address VARCHAR(300),
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    CONSTRAINT suppliers_status_check CHECK (status IN ('ACTIVE', 'INACTIVE'))
);

CREATE UNIQUE INDEX suppliers_name_uq ON suppliers (organization_id, LOWER(name));
CREATE UNIQUE INDEX suppliers_tax_id_uq ON suppliers (organization_id, tax_id) WHERE tax_id IS NOT NULL;

-- Catálogo de productos o insumos, agrupados opcionalmente por categoría.
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(id),
    name VARCHAR(200) NOT NULL,
    category VARCHAR(100),
    unit VARCHAR(24) NOT NULL,
    requires_temperature BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    CONSTRAINT products_status_check CHECK (status IN ('ACTIVE', 'INACTIVE'))
);

CREATE UNIQUE INDEX products_name_uq ON products (organization_id, LOWER(name));

-- Recepciones de insumos, con referencia opcional a guía, factura u otro documento.
CREATE TABLE supply_receipts (
    id UUID PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL,
    location_id UUID NOT NULL,
    supplier_id UUID NOT NULL,
    received_by VARCHAR(128) NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    document_reference VARCHAR(100),
    notes TEXT,
    client_operation_id UUID NOT NULL,
    client_created_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, client_operation_id),
    FOREIGN KEY (organization_id, location_id)
        REFERENCES locations(organization_id, id),
    FOREIGN KEY (organization_id, supplier_id)
        REFERENCES suppliers(organization_id, id),
    FOREIGN KEY (organization_id, received_by)
        REFERENCES organization_memberships(organization_id, user_id)
);

-- Productos, lotes, vencimientos, temperaturas y decisiones de cada recepción.
CREATE TABLE supply_receipt_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    receipt_id UUID NOT NULL,
    product_id UUID NOT NULL,
    batch_code VARCHAR(100),
    expiration_date DATE,
    quantity NUMERIC(12, 3) NOT NULL,
    temperature_celsius NUMERIC(6, 2),
    decision VARCHAR(20) NOT NULL,
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, receipt_id)
        REFERENCES supply_receipts(organization_id, id),
    FOREIGN KEY (organization_id, product_id)
        REFERENCES products(organization_id, id),
    CONSTRAINT supply_receipt_items_quantity_check CHECK (quantity > 0),
    CONSTRAINT supply_receipt_items_decision_check CHECK (decision IN ('ACCEPTED', 'REJECTED', 'OBSERVED')),
    CONSTRAINT supply_receipt_items_rejection_check
        CHECK (decision <> 'REJECTED' OR rejection_reason IS NOT NULL)
);

-- Lotes almacenados y su ubicación para control de disponibilidad y rotación PEPS o FIFO.
CREATE TABLE storage_lots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    location_id UUID NOT NULL,
    receipt_item_id UUID NOT NULL,
    storage_location VARCHAR(120) NOT NULL,
    label VARCHAR(160),
    quantity_available NUMERIC(12, 3) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'AVAILABLE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, location_id)
        REFERENCES locations(organization_id, id),
    FOREIGN KEY (organization_id, receipt_item_id)
        REFERENCES supply_receipt_items(organization_id, id),
    CONSTRAINT storage_lots_quantity_check CHECK (quantity_available >= 0),
    CONSTRAINT storage_lots_status_check CHECK (status IN ('AVAILABLE', 'CONSUMED', 'DISCARDED'))
);

-- Planes de limpieza por sede y área, con instrucciones operativas opcionales.
CREATE TABLE cleaning_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    location_id UUID NOT NULL,
    area VARCHAR(100) NOT NULL,
    task VARCHAR(240) NOT NULL,
    instructions TEXT,
    product_name VARCHAR(160),
    frequency VARCHAR(20) NOT NULL,
    due_time TIME,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, location_id)
        REFERENCES locations(organization_id, id),
    CONSTRAINT cleaning_plans_frequency_check
        CHECK (frequency IN ('PER_SHIFT', 'DAILY', 'WEEKLY', 'MONTHLY'))
);

-- Ejecuciones de actividades de limpieza realizadas por el personal responsable.
CREATE TABLE cleaning_executions (
    id UUID PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL,
    location_id UUID NOT NULL,
    cleaning_plan_id UUID NOT NULL,
    responsible_user_id VARCHAR(128) NOT NULL,
    executed_at TIMESTAMPTZ NOT NULL,
    result VARCHAR(20) NOT NULL,
    observation TEXT,
    client_operation_id UUID NOT NULL,
    client_created_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, client_operation_id),
    FOREIGN KEY (organization_id, location_id)
        REFERENCES locations(organization_id, id),
    FOREIGN KEY (organization_id, cleaning_plan_id)
        REFERENCES cleaning_plans(organization_id, id),
    FOREIGN KEY (organization_id, responsible_user_id)
        REFERENCES organization_memberships(organization_id, user_id),
    CONSTRAINT cleaning_executions_result_check CHECK (result IN ('COMPLETED', 'NOT_COMPLETED', 'OBSERVED'))
);

-- Evidencias multimedia asociadas a una ejecución de limpieza.
CREATE TABLE cleaning_media (
    organization_id VARCHAR(64) NOT NULL,
    cleaning_execution_id UUID NOT NULL,
    media_file_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    PRIMARY KEY (organization_id, cleaning_execution_id, media_file_id),
    FOREIGN KEY (organization_id, cleaning_execution_id)
        REFERENCES cleaning_executions(organization_id, id),
    FOREIGN KEY (organization_id, media_file_id)
        REFERENCES media_files(organization_id, id)
);

-- Catálogo configurable de incidencias BPM, área, nivel de riesgo y referencia normativa.
CREATE TABLE incident_catalog (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(id),
    code VARCHAR(40) NOT NULL,
    name VARCHAR(200) NOT NULL,
    area VARCHAR(100) NOT NULL,
    normative_criterion TEXT NOT NULL,
    risk_level VARCHAR(20) NOT NULL,
    evidence_required BOOLEAN NOT NULL DEFAULT TRUE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, code),
    CONSTRAINT incident_catalog_risk_check CHECK (risk_level IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL'))
);

-- Incidencias detectadas en una sede y su estado de atención.
CREATE TABLE incidents (
    id UUID PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL,
    location_id UUID NOT NULL,
    catalog_id UUID NOT NULL,
    reported_by VARCHAR(128) NOT NULL,
    description TEXT,
    status VARCHAR(24) NOT NULL DEFAULT 'OPEN',
    client_operation_id UUID NOT NULL,
    client_created_at TIMESTAMPTZ NOT NULL,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, client_operation_id),
    FOREIGN KEY (organization_id, location_id)
        REFERENCES locations(organization_id, id),
    FOREIGN KEY (organization_id, catalog_id)
        REFERENCES incident_catalog(organization_id, id),
    FOREIGN KEY (organization_id, reported_by)
        REFERENCES organization_memberships(organization_id, user_id),
    CONSTRAINT incidents_status_check
        CHECK (status IN ('OPEN', 'ASSIGNED', 'IN_PROGRESS', 'RESOLVED', 'VERIFIED', 'CLOSED'))
);

-- Acciones correctivas asignadas para resolver y verificar una incidencia.
CREATE TABLE corrective_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    incident_id UUID NOT NULL,
    responsible_user_id VARCHAR(128) NOT NULL,
    description TEXT NOT NULL,
    due_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'PENDING',
    resolved_at TIMESTAMPTZ,
    verified_at TIMESTAMPTZ,
    verified_by VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, incident_id)
        REFERENCES incidents(organization_id, id),
    FOREIGN KEY (organization_id, responsible_user_id)
        REFERENCES organization_memberships(organization_id, user_id),
    CONSTRAINT corrective_actions_status_check
        CHECK (status IN ('PENDING', 'IN_PROGRESS', 'RESOLVED', 'VERIFIED', 'CANCELLED'))
);

-- Evidencias antes, después o generales asociadas a incidencias y acciones correctivas.
CREATE TABLE incident_media (
    organization_id VARCHAR(64) NOT NULL,
    incident_id UUID NOT NULL,
    corrective_action_id UUID,
    media_file_id UUID NOT NULL,
    stage VARCHAR(20) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    PRIMARY KEY (organization_id, incident_id, media_file_id),
    FOREIGN KEY (organization_id, incident_id)
        REFERENCES incidents(organization_id, id),
    FOREIGN KEY (organization_id, corrective_action_id)
        REFERENCES corrective_actions(organization_id, id),
    FOREIGN KEY (organization_id, media_file_id)
        REFERENCES media_files(organization_id, id),
    CONSTRAINT incident_media_stage_check CHECK (stage IN ('BEFORE', 'AFTER', 'GENERAL'))
);

CREATE INDEX supply_receipts_location_date_idx ON supply_receipts (organization_id, location_id, received_at DESC);
CREATE INDEX receipt_items_expiration_idx ON supply_receipt_items (organization_id, expiration_date) WHERE decision <> 'REJECTED';
CREATE INDEX storage_lots_fifo_idx ON storage_lots (organization_id, location_id, status, created_at);
CREATE INDEX cleaning_plans_location_active_idx ON cleaning_plans (organization_id, location_id, active);
CREATE INDEX cleaning_executions_location_date_idx ON cleaning_executions (organization_id, location_id, executed_at DESC);
CREATE INDEX incidents_location_status_idx ON incidents (organization_id, location_id, status, created_at DESC);
CREATE INDEX corrective_actions_due_idx ON corrective_actions (organization_id, status, due_at);

CREATE TRIGGER suppliers_set_updated_at BEFORE UPDATE ON suppliers FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER products_set_updated_at BEFORE UPDATE ON products FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER supply_receipts_set_updated_at BEFORE UPDATE ON supply_receipts FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER supply_receipt_items_set_updated_at BEFORE UPDATE ON supply_receipt_items FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER storage_lots_set_updated_at BEFORE UPDATE ON storage_lots FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER cleaning_plans_set_updated_at BEFORE UPDATE ON cleaning_plans FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER cleaning_executions_set_updated_at BEFORE UPDATE ON cleaning_executions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER cleaning_media_set_updated_at BEFORE UPDATE ON cleaning_media FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER incident_catalog_set_updated_at BEFORE UPDATE ON incident_catalog FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER incidents_set_updated_at BEFORE UPDATE ON incidents FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER corrective_actions_set_updated_at BEFORE UPDATE ON corrective_actions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER incident_media_set_updated_at BEFORE UPDATE ON incident_media FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================================
-- 4. ACADEMIA BPM
-- ============================================================================

-- Cursos BPM con descripción y duración estimada para el usuario.
CREATE TABLE training_courses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(id),
    code VARCHAR(50) NOT NULL,
    title VARCHAR(200) NOT NULL,
    description TEXT,
    estimated_duration_minutes INTEGER,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, code),
    CONSTRAINT training_courses_duration_check
        CHECK (estimated_duration_minutes IS NULL OR estimated_duration_minutes > 0),
    CONSTRAINT training_courses_status_check CHECK (status IN ('DRAFT', 'PUBLISHED', 'ARCHIVED'))
);

-- Versiones inmutables o en preparación del contenido y criterio de aprobación de un curso.
CREATE TABLE training_course_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    course_id UUID NOT NULL,
    version_number INTEGER NOT NULL,
    passing_score INTEGER NOT NULL DEFAULT 80,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, course_id, version_number),
    FOREIGN KEY (organization_id, course_id)
        REFERENCES training_courses(organization_id, id),
    CONSTRAINT training_course_versions_number_check CHECK (version_number > 0),
    CONSTRAINT training_course_versions_score_check CHECK (passing_score BETWEEN 0 AND 100),
    CONSTRAINT training_course_versions_status_check CHECK (status IN ('DRAFT', 'PUBLISHED', 'ARCHIVED'))
);

-- Lecciones ordenadas que forman parte de una versión de curso.
CREATE TABLE training_lessons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    course_version_id UUID NOT NULL,
    title VARCHAR(200) NOT NULL,
    content TEXT NOT NULL,
    display_order INTEGER NOT NULL,
    media_file_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (course_version_id, display_order),
    FOREIGN KEY (organization_id, course_version_id)
        REFERENCES training_course_versions(organization_id, id),
    FOREIGN KEY (organization_id, media_file_id)
        REFERENCES media_files(organization_id, id),
    CONSTRAINT training_lessons_order_check CHECK (display_order > 0)
);

-- Preguntas evaluativas y alternativas de respuesta de una versión o lección.
CREATE TABLE training_questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    course_version_id UUID NOT NULL,
    lesson_id UUID,
    question_text TEXT NOT NULL,
    options JSONB NOT NULL,
    correct_option INTEGER NOT NULL,
    explanation TEXT,
    display_order INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, course_version_id)
        REFERENCES training_course_versions(organization_id, id),
    FOREIGN KEY (organization_id, lesson_id)
        REFERENCES training_lessons(organization_id, id),
    CONSTRAINT training_questions_options_check CHECK (jsonb_typeof(options) = 'array'),
    CONSTRAINT training_questions_option_check CHECK (correct_option >= 0),
    CONSTRAINT training_questions_order_check CHECK (display_order > 0)
);

-- Asignaciones de cursos a usuarios, roles o sedes con fecha límite opcional.
CREATE TABLE training_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    location_id UUID,
    course_id UUID NOT NULL,
    target_user_id VARCHAR(128),
    target_role VARCHAR(20),
    due_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, location_id)
        REFERENCES locations(organization_id, id),
    FOREIGN KEY (organization_id, course_id)
        REFERENCES training_courses(organization_id, id),
    FOREIGN KEY (organization_id, target_user_id)
        REFERENCES organization_memberships(organization_id, user_id),
    CONSTRAINT training_assignments_target_check
        CHECK (target_user_id IS NOT NULL OR target_role IS NOT NULL),
    CONSTRAINT training_assignments_role_check
        CHECK (target_role IS NULL OR target_role IN ('ADMIN', 'CHEF', 'WORKER')),
    CONSTRAINT training_assignments_status_check CHECK (status IN ('ACTIVE', 'COMPLETED', 'CANCELLED'))
);

-- Intentos diagnósticos, modulares o finales realizados por usuarios, incluso sin conexión.
CREATE TABLE training_attempts (
    id UUID PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL,
    course_version_id UUID NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    attempt_type VARCHAR(20) NOT NULL,
    score INTEGER,
    passed BOOLEAN,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    client_operation_id UUID NOT NULL,
    client_created_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, client_operation_id),
    FOREIGN KEY (organization_id, course_version_id)
        REFERENCES training_course_versions(organization_id, id),
    FOREIGN KEY (organization_id, user_id)
        REFERENCES organization_memberships(organization_id, user_id),
    CONSTRAINT training_attempts_type_check CHECK (attempt_type IN ('DIAGNOSTIC', 'MODULE', 'FINAL')),
    CONSTRAINT training_attempts_score_check CHECK (score IS NULL OR score BETWEEN 0 AND 100)
);

-- Respuestas seleccionadas en un intento de evaluación y su resultado calculado.
CREATE TABLE training_attempt_answers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    attempt_id UUID NOT NULL,
    question_id UUID NOT NULL,
    selected_option INTEGER NOT NULL,
    is_correct BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (attempt_id, question_id),
    FOREIGN KEY (organization_id, attempt_id)
        REFERENCES training_attempts(organization_id, id),
    FOREIGN KEY (organization_id, question_id)
        REFERENCES training_questions(organization_id, id),
    CONSTRAINT training_attempt_answers_option_check CHECK (selected_option >= 0)
);

-- Resumen del avance, mejor puntaje y experiencia de un usuario por curso.
CREATE TABLE training_progress (
    organization_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    course_id UUID NOT NULL,
    last_lesson_id UUID,
    status VARCHAR(20) NOT NULL DEFAULT 'NOT_STARTED',
    best_score INTEGER,
    experience_points INTEGER NOT NULL DEFAULT 0,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    PRIMARY KEY (organization_id, user_id, course_id),
    FOREIGN KEY (organization_id, user_id)
        REFERENCES organization_memberships(organization_id, user_id),
    FOREIGN KEY (organization_id, course_id)
        REFERENCES training_courses(organization_id, id),
    FOREIGN KEY (organization_id, last_lesson_id)
        REFERENCES training_lessons(organization_id, id),
    CONSTRAINT training_progress_status_check
        CHECK (status IN ('NOT_STARTED', 'IN_PROGRESS', 'COMPLETED')),
    CONSTRAINT training_progress_score_check CHECK (best_score IS NULL OR best_score BETWEEN 0 AND 100),
    CONSTRAINT training_progress_points_check CHECK (experience_points >= 0)
);

-- Constancias internas emitidas por aprobaciones verificables de capacitación.
CREATE TABLE training_certificates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    course_version_id UUID NOT NULL,
    final_attempt_id UUID NOT NULL,
    verification_code VARCHAR(64) NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (verification_code),
    FOREIGN KEY (organization_id, user_id)
        REFERENCES organization_memberships(organization_id, user_id),
    FOREIGN KEY (organization_id, course_version_id)
        REFERENCES training_course_versions(organization_id, id),
    FOREIGN KEY (organization_id, final_attempt_id)
        REFERENCES training_attempts(organization_id, id)
);

-- Catálogo de logros o insignias y los puntos que otorgan.
CREATE TABLE training_achievements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(id),
    code VARCHAR(50) NOT NULL,
    name VARCHAR(120) NOT NULL,
    description VARCHAR(300),
    points INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, code),
    CONSTRAINT training_achievements_points_check CHECK (points >= 0)
);

-- Logros obtenidos por cada usuario dentro de una organización.
CREATE TABLE user_achievements (
    organization_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    achievement_id UUID NOT NULL,
    awarded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    PRIMARY KEY (organization_id, user_id, achievement_id),
    FOREIGN KEY (organization_id, user_id)
        REFERENCES organization_memberships(organization_id, user_id),
    FOREIGN KEY (organization_id, achievement_id)
        REFERENCES training_achievements(organization_id, id)
);

CREATE INDEX training_courses_tenant_status_idx ON training_courses (organization_id, status);
CREATE INDEX training_lessons_version_order_idx ON training_lessons (course_version_id, display_order);
CREATE INDEX training_questions_version_order_idx ON training_questions (course_version_id, display_order);
CREATE INDEX training_assignments_target_idx ON training_assignments (organization_id, target_user_id, status);
CREATE INDEX training_attempts_user_date_idx ON training_attempts (organization_id, user_id, created_at DESC);
CREATE INDEX training_progress_company_idx ON training_progress (organization_id, status, updated_at DESC);

CREATE TRIGGER training_courses_set_updated_at BEFORE UPDATE ON training_courses FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER training_course_versions_set_updated_at BEFORE UPDATE ON training_course_versions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER training_lessons_set_updated_at BEFORE UPDATE ON training_lessons FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER training_questions_set_updated_at BEFORE UPDATE ON training_questions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER training_assignments_set_updated_at BEFORE UPDATE ON training_assignments FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER training_attempts_set_updated_at BEFORE UPDATE ON training_attempts FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER training_attempt_answers_set_updated_at BEFORE UPDATE ON training_attempt_answers FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER training_progress_set_updated_at BEFORE UPDATE ON training_progress FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER training_certificates_set_updated_at BEFORE UPDATE ON training_certificates FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER training_achievements_set_updated_at BEFORE UPDATE ON training_achievements FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER user_achievements_set_updated_at BEFORE UPDATE ON user_achievements FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================================
-- 5. INVESTIGACIÓN, ALERTAS Y USO
-- ============================================================================

-- Participantes seudonimizados del estudio y su relación opcional con usuario y sede.
CREATE TABLE research_participants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    location_id UUID,
    user_id VARCHAR(128),
    participant_code VARCHAR(60) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, participant_code),
    FOREIGN KEY (organization_id, location_id)
        REFERENCES locations(organization_id, id),
    FOREIGN KEY (organization_id, user_id)
        REFERENCES organization_memberships(organization_id, user_id),
    CONSTRAINT research_participants_status_check CHECK (status IN ('ACTIVE', 'WITHDRAWN', 'COMPLETED'))
);

-- Registro de aceptación o rechazo del consentimiento y versión del documento presentado.
CREATE TABLE consent_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    participant_id UUID NOT NULL,
    document_version VARCHAR(40) NOT NULL,
    accepted BOOLEAN NOT NULL,
    accepted_at TIMESTAMPTZ NOT NULL,
    recorded_by VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, participant_id)
        REFERENCES research_participants(organization_id, id)
);

-- Instrumentos globales de investigación con una descripción breve de su propósito.
CREATE TABLE survey_instruments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    CONSTRAINT survey_instruments_status_check CHECK (status IN ('DRAFT', 'PUBLISHED', 'ARCHIVED'))
);

-- Versiones publicadas o en preparación de un instrumento de investigación.
CREATE TABLE survey_instrument_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instrument_id UUID NOT NULL REFERENCES survey_instruments(id),
    version_number INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (instrument_id, version_number),
    CONSTRAINT survey_instrument_versions_number_check CHECK (version_number > 0),
    CONSTRAINT survey_instrument_versions_status_check CHECK (status IN ('DRAFT', 'PUBLISHED', 'ARCHIVED'))
);

-- Preguntas, dimensiones, tipos y opciones de una versión de instrumento.
CREATE TABLE survey_questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instrument_version_id UUID NOT NULL REFERENCES survey_instrument_versions(id),
    section VARCHAR(40) NOT NULL,
    question_text TEXT NOT NULL,
    answer_type VARCHAR(24) NOT NULL,
    options JSONB,
    display_order INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (instrument_version_id, display_order),
    CONSTRAINT survey_questions_type_check
        CHECK (answer_type IN ('MULTIPLE_CHOICE', 'LIKERT', 'FREQUENCY', 'TEXT', 'NUMBER')),
    CONSTRAINT survey_questions_order_check CHECK (display_order > 0),
    CONSTRAINT survey_questions_options_check CHECK (options IS NULL OR jsonb_typeof(options) = 'array')
);

-- Aplicaciones pretest o postest entregadas por un participante.
CREATE TABLE survey_submissions (
    id UUID PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL,
    participant_id UUID NOT NULL,
    instrument_version_id UUID NOT NULL REFERENCES survey_instrument_versions(id),
    moment VARCHAR(12) NOT NULL,
    submitted_at TIMESTAMPTZ,
    client_operation_id UUID NOT NULL,
    client_created_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    UNIQUE (organization_id, participant_id, instrument_version_id, moment),
    UNIQUE (organization_id, client_operation_id),
    FOREIGN KEY (organization_id, participant_id)
        REFERENCES research_participants(organization_id, id),
    CONSTRAINT survey_submissions_moment_check CHECK (moment IN ('PRETEST', 'POSTTEST'))
);

-- Respuestas textuales o numéricas pertenecientes a una entrega de encuesta.
CREATE TABLE survey_answers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    submission_id UUID NOT NULL,
    question_id UUID NOT NULL REFERENCES survey_questions(id),
    answer_text TEXT,
    answer_number NUMERIC(12, 3),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (submission_id, question_id),
    FOREIGN KEY (organization_id, submission_id)
        REFERENCES survey_submissions(organization_id, id),
    CONSTRAINT survey_answers_value_check
        CHECK (
            (answer_text IS NOT NULL AND answer_number IS NULL)
            OR (answer_text IS NULL AND answer_number IS NOT NULL)
        )
);

-- Reglas de alerta identificables por nombre y configurables por organización o sede.
CREATE TABLE alert_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(id),
    location_id UUID,
    name VARCHAR(160) NOT NULL,
    alert_type VARCHAR(40) NOT NULL,
    configuration JSONB NOT NULL DEFAULT '{}'::jsonb,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, location_id)
        REFERENCES locations(organization_id, id)
);

-- Alertas generadas para usuarios y su estado de envío, visualización o atención.
CREATE TABLE alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    location_id UUID,
    recipient_user_id VARCHAR(128) NOT NULL,
    alert_type VARCHAR(40) NOT NULL,
    title VARCHAR(160) NOT NULL,
    message TEXT NOT NULL,
    source_type VARCHAR(60),
    source_id VARCHAR(128),
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    viewed_at TIMESTAMPTZ,
    attended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, location_id)
        REFERENCES locations(organization_id, id),
    FOREIGN KEY (organization_id, recipient_user_id)
        REFERENCES organization_memberships(organization_id, user_id),
    CONSTRAINT alerts_status_check CHECK (status IN ('PENDING', 'SENT', 'VIEWED', 'ATTENDED', 'CANCELLED'))
);

-- Intentos y resultados de entrega de alertas mediante FCM o correo.
CREATE TABLE notification_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    alert_id UUID NOT NULL,
    channel VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    provider_message_id VARCHAR(200),
    attempts INTEGER NOT NULL DEFAULT 0,
    sent_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    FOREIGN KEY (organization_id, alert_id)
        REFERENCES alerts(organization_id, id),
    CONSTRAINT notification_deliveries_channel_check CHECK (channel IN ('FCM', 'EMAIL')),
    CONSTRAINT notification_deliveries_status_check
        CHECK (status IN ('PENDING', 'SENT', 'DELIVERED', 'FAILED')),
    CONSTRAINT notification_deliveries_attempts_check CHECK (attempts >= 0)
);

-- Sesiones de uso de la aplicación por usuario, organización y dispositivo.
CREATE TABLE usage_sessions (
    id UUID PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    device_id UUID,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    UNIQUE (organization_id, id),
    FOREIGN KEY (organization_id, user_id)
        REFERENCES organization_memberships(organization_id, user_id),
    FOREIGN KEY (organization_id, device_id)
        REFERENCES mobile_devices(organization_id, id)
);

-- Eventos de navegación o interacción registrados dentro de una sesión de uso.
CREATE TABLE usage_events (
    id BIGSERIAL PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL,
    session_id UUID NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    module VARCHAR(60) NOT NULL,
    event_name VARCHAR(80) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    duration_seconds INTEGER,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128),
    updated_by VARCHAR(128),
    FOREIGN KEY (organization_id, session_id)
        REFERENCES usage_sessions(organization_id, id),
    FOREIGN KEY (organization_id, user_id)
        REFERENCES organization_memberships(organization_id, user_id),
    CONSTRAINT usage_events_duration_check CHECK (duration_seconds IS NULL OR duration_seconds >= 0)
);

CREATE INDEX research_participants_tenant_status_idx ON research_participants (organization_id, status);
CREATE INDEX survey_submissions_tenant_moment_idx ON survey_submissions (organization_id, moment, submitted_at DESC);
CREATE INDEX alerts_recipient_status_idx ON alerts (organization_id, recipient_user_id, status, created_at DESC);
CREATE INDEX alerts_source_idx ON alerts (organization_id, source_type, source_id);
CREATE INDEX notification_deliveries_pending_idx ON notification_deliveries (status, created_at) WHERE status IN ('PENDING', 'FAILED');
CREATE INDEX usage_sessions_user_date_idx ON usage_sessions (organization_id, user_id, started_at DESC);
CREATE INDEX usage_events_tenant_time_idx ON usage_events (organization_id, occurred_at DESC);
CREATE INDEX usage_events_module_idx ON usage_events (organization_id, module, event_name, occurred_at DESC);

-- Cumplimiento diario promedio de checklists enviados, agrupado por organización y sede.
CREATE VIEW daily_checklist_compliance AS
SELECT
    r.organization_id,
    r.location_id,
    (r.client_created_at AT TIME ZONE l.timezone)::date AS local_date,
    COUNT(*) AS completed_checklists,
    ROUND(AVG(r.compliance_percent), 2) AS average_compliance
FROM checklist_runs r
JOIN locations l
  ON l.organization_id = r.organization_id
 AND l.id = r.location_id
WHERE r.status = 'SUBMITTED'
GROUP BY r.organization_id, r.location_id, (r.client_created_at AT TIME ZONE l.timezone)::date;

-- Resumen por organización de cursos completados, pendientes y puntaje promedio.
CREATE VIEW organization_training_summary AS
SELECT
    organization_id,
    COUNT(*) FILTER (WHERE status = 'COMPLETED') AS completed_courses,
    COUNT(*) FILTER (WHERE status <> 'COMPLETED') AS pending_courses,
    ROUND(AVG(best_score), 2) AS average_score
FROM training_progress
GROUP BY organization_id;

CREATE TRIGGER research_participants_set_updated_at BEFORE UPDATE ON research_participants FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER consent_records_set_updated_at BEFORE UPDATE ON consent_records FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER survey_instruments_set_updated_at BEFORE UPDATE ON survey_instruments FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER survey_instrument_versions_set_updated_at BEFORE UPDATE ON survey_instrument_versions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER survey_questions_set_updated_at BEFORE UPDATE ON survey_questions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER survey_submissions_set_updated_at BEFORE UPDATE ON survey_submissions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER survey_answers_set_updated_at BEFORE UPDATE ON survey_answers FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER alert_rules_set_updated_at BEFORE UPDATE ON alert_rules FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER alerts_set_updated_at BEFORE UPDATE ON alerts FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER notification_deliveries_set_updated_at BEFORE UPDATE ON notification_deliveries FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER usage_sessions_set_updated_at BEFORE UPDATE ON usage_sessions FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER usage_events_set_updated_at BEFORE UPDATE ON usage_events FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;

