-- Vuelve a la vista de la migración 002 (solo reportes). Los datasets
-- importados se pierden.
DROP VIEW IF EXISTS riesgo_historico;
DROP TABLE IF EXISTS incidente_historico;
DROP TABLE IF EXISTS dataset_historico;

CREATE VIEW reporte_historico_peso AS
SELECT r.id AS reporte_id,
       r.incidente_id,
       r.created_at,
       p.geom,
       NULLIF(btrim(split_part(r.direccion, ' · ', 2)), '')::text AS colonia,
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
AND r.estado_actual <> 'Descartado';
