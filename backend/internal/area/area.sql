-- name: ListReportesEnArea :many
-- sqlc no conoce PostGIS: el polígono viaja como texto GeoJSON y se convierte en el servidor.
-- ST_Covers sobre geography aprovecha el índice GiST de puntos_origen.geom.
-- Mismas columnas que ListReportesResumen para reutilizar el resumen del mapa.
SELECT r.id, r.incidente_id, r.estado_actual, r.avistamientos, r.es_oficial, p.latitude, p.longitude,
       r.direccion, i.etiqueta AS incidente_etiqueta, r.created_at
FROM reporte r
JOIN puntos_origen p ON p.reporte_id = r.id
JOIN incidentes i ON i.id = r.incidente_id
WHERE NOT r.es_historico
AND ST_Covers(ST_GeomFromGeoJSON(sqlc.arg(poligono)::text)::geography, p.geom)
ORDER BY r.created_at;

-- name: ListCentrosEnArea :many
-- Solo activos, igual que la lista del mapa. Mismas columnas que ListCentros.
SELECT id, tipo_centro, nombre, descripcion, image_path,
       latitude, longitude, capacidad, cantidad, activo, created_at
FROM centros
WHERE activo
AND ST_Covers(ST_GeomFromGeoJSON(sqlc.arg(poligono)::text)::geography, geom)
ORDER BY nombre;
