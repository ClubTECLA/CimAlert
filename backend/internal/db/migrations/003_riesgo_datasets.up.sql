-- Datasets históricos importados (ver backend/GUIA_RIESGO.md, "Datasets").
--
-- El riesgo lee de dos fuentes:
-- - reporte con es_historico (lo marca a mano un rol oficial, migración 002), y
-- - incidente_historico: filas de un archivo .xlsx o .csv que se importa.
-- La vista riesgo_historico junta las dos con las mismas columnas y reemplaza
-- a reporte_historico_peso.

-- Un archivo importado. Borrarlo borra sus filas (ON DELETE CASCADE).
CREATE TABLE dataset_historico (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    nombre text NOT NULL,
    -- Nombre original del archivo subido.
    archivo text NOT NULL,
    -- SHA-256 del contenido: el mismo archivo no se importa dos veces.
    sha256 text NOT NULL UNIQUE,
    filas integer NOT NULL CHECK (filas > 0),
    -- Quién lo importó; si se borra el usuario el dataset se conserva.
    cargado_por uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz DEFAULT now() NOT NULL
);

-- Una fila del archivo: un incidente histórico.
CREATE TABLE incidente_historico (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    dataset_id uuid NOT NULL REFERENCES dataset_historico(id) ON DELETE CASCADE,
    -- Número de renglón en el archivo (el encabezado es el 1), para rastrear el dato.
    fila integer NOT NULL,
    incidente_id integer NOT NULL REFERENCES incidentes(id),
    colonia text,
    -- El formato solo trae mes y año: se guarda el día 1 del mes.
    fecha date NOT NULL,
    latitude double precision NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude double precision NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    -- NULL = el archivo no traía el dato.
    personas_afectadas integer CHECK (personas_afectadas >= 0),
    fallecidos integer CHECK (fallecidos >= 0),
    geom geography(Point, 4326) GENERATED ALWAYS AS (
        ST_SetSRID(ST_MakePoint(longitude, latitude), 4326)::geography
    ) STORED
);

CREATE INDEX incidente_historico_geom_idx ON incidente_historico USING gist (geom);
CREATE INDEX incidente_historico_dataset_idx ON incidente_historico (dataset_id);

DROP VIEW reporte_historico_peso;

-- Incidentes históricos que cuentan para el riesgo, con su peso. Es la única
-- fuente de datos de la funcionalidad riesgo. Cambiar la fórmula es una
-- migración nueva y subir riesgo.VersionModelo (versión 2).
CREATE VIEW riesgo_historico AS
-- Reportes de la app marcados como históricos (sin descartados: requerimiento R-10).
SELECT 'reporte'::text AS origen,
       r.id AS reporte_id,
       NULL::uuid AS dataset_id,
       r.incidente_id,
       r.created_at,
       p.geom,
       -- direccion viene como "calle · colonia" (geocodificación del cliente);
       -- sin dirección o sin la parte de la colonia queda NULL. El ::text es
       -- para sqlc, que no infiere el tipo de NULLIF.
       NULLIF(btrim(split_part(r.direccion, ' · ', 2)), '')::text AS colonia,
       NULL::integer AS personas_afectadas,
       NULL::integer AS fallecidos,
       -- credibilidad del estado × confirmación por avistamientos (logarítmica:
       -- el décimo avistamiento aporta menos que el primero).
       ((CASE r.estado_actual
             WHEN 'Verificado' THEN 1.0
             WHEN 'Resuelto'   THEN 1.0
             WHEN 'Probable'   THEN 0.75
             ELSE 0.5 -- Pendiente
         END)
        * (1 + ln(1 + r.avistamientos)))::float8 AS peso
FROM reporte r
JOIN puntos_origen p ON p.reporte_id = r.id
WHERE r.es_historico
AND r.estado_actual <> 'Descartado'
UNION ALL
-- Filas de los datasets importados: dato oficial (credibilidad 1.0) ×
-- personas afectadas con el mismo logaritmo que los avistamientos.
SELECT 'dataset'::text,
       NULL::uuid,
       h.dataset_id,
       h.incidente_id,
       -- Mediodía en Tijuana: el día 1 no cambia de mes al pasar a UTC.
       ((h.fecha + time '12:00') AT TIME ZONE 'America/Tijuana')::timestamptz,
       h.geom,
       h.colonia,
       h.personas_afectadas,
       h.fallecidos,
       (1.0 * (1 + ln(1 + COALESCE(h.personas_afectadas, 0))))::float8
FROM incidente_historico h;
