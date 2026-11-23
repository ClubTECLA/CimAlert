import { useCallback, useEffect, useMemo, useRef } from "react";
import type { GeoJSONSource, Map as MapLibreMap, MapMouseEvent } from "maplibre-gl";
import type { Feature, FeatureCollection, LineString, Point, Polygon, Position } from "geojson";
import { polygonFromPoints } from "@/lib/geo";
import { setupMapLayers } from "./mapLayers";
import { useMap } from "./useMap";
import { usePolygon } from "./usePolygon";

// Capas propias del área seleccionada: el relleno y el borde del polígono (ya aplicado o en dibujo), el último
// segmento que sigue al cursor y los vértices. El primer vértice va en otro color: clic en él cierra el anillo.
const AREA_SOURCE = "area-seleccionada";
const VERTEX_SOURCE = "area-seleccionada-vertices";
const AREA_FILL = "area-seleccionada-relleno";
const AREA_LINE = "area-seleccionada-borde";
const DRAFT_LINE = "area-seleccionada-linea";
const VERTEX_CIRCLES = "area-seleccionada-vertice";

const BRAND = "#2677e6";
const MINT = "#00c8b3";
// Radio en píxeles alrededor del primer vértice para cerrar haciendo clic en él.
const CLOSE_HIT_RADIUS = 14;
const EMPTY_VERTICES: FeatureCollection<Point, { first: boolean }> = { type: "FeatureCollection", features: [] };
type LayerData = { area: FeatureCollection; vertices: FeatureCollection<Point, { first: boolean }> };

function buildArea(draft: Position[], polygon: Polygon | null, hover: Position | null, isDrawing: boolean): FeatureCollection {
    const features: Feature<Polygon | LineString>[] = [];
    const area = isDrawing ? polygonFromPoints(draft) : polygon;
    if (area) features.push({ type: "Feature", geometry: area, properties: {} });

    if (isDrawing && draft.length) {
        const last = draft[draft.length - 1];
        const tip = hover ?? last;
        const points = tip[0] === last[0] && tip[1] === last[1] ? draft : [...draft, tip];
        if (points.length >= 2) {
            features.push({ type: "Feature", geometry: { type: "LineString", coordinates: points }, properties: {} });
        }
    }
    return { type: "FeatureCollection", features };
}

function buildVertices(draft: Position[], isDrawing: boolean): FeatureCollection<Point, { first: boolean }> {
    if (!isDrawing) return EMPTY_VERTICES;
    return {
        type: "FeatureCollection",
        features: draft.map((point, index) => ({
            type: "Feature",
            geometry: { type: "Point", coordinates: point },
            properties: { first: index === 0 },
        })),
    };
}

function setupLayers(map: MapLibreMap, data: LayerData) {
    map.addSource(AREA_SOURCE, { type: "geojson", data: data.area });
    map.addSource(VERTEX_SOURCE, { type: "geojson", data: data.vertices });

    map.addLayer({
        id: AREA_FILL,
        type: "fill",
        source: AREA_SOURCE,
        filter: ["==", ["geometry-type"], "Polygon"],
        paint: { "fill-color": BRAND, "fill-opacity": 0.14 },
    });
    map.addLayer({
        id: AREA_LINE,
        type: "line",
        source: AREA_SOURCE,
        filter: ["==", ["geometry-type"], "Polygon"],
        paint: { "line-color": BRAND, "line-width": 2.5, "line-dasharray": [3, 1] },
    });
    map.addLayer({
        id: DRAFT_LINE,
        type: "line",
        source: AREA_SOURCE,
        filter: ["==", ["geometry-type"], "LineString"],
        paint: { "line-color": BRAND, "line-width": 2, "line-dasharray": [2, 1] },
    });
    map.addLayer({
        id: VERTEX_CIRCLES,
        type: "circle",
        source: VERTEX_SOURCE,
        paint: {
            "circle-radius": ["case", ["get", "first"], 7, 5],
            "circle-color": "#ffffff",
            "circle-stroke-width": 2,
            "circle-stroke-color": ["case", ["get", "first"], MINT, BRAND],
        },
    });
}

/**
 * Dibujo del polígono sobre el mapa. Los listeners se registran una sola vez y leen las acciones del contexto desde
 * un ref: así el efecto no se re-suscribe cuando cambia el dibujo ni cuando llegan reportes nuevos del sondeo.
 */
export default function PolygonLayer() {
    const { map } = useMap();
    const { polygon, isDrawing, draft, addPoint, finish, cancel } = usePolygon();

    const hoverRef = useRef<Position | null>(null);
    // Datos con los que nacen las fuentes: si MapLibre las crea en un styledata posterior, ya reciben el estado
    // vigente en vez de una colección vacía.
    const currentData = useMemo<LayerData>(() => ({
        area: buildArea(draft, polygon, null, isDrawing),
        vertices: buildVertices(draft, isDrawing),
    }), [draft, polygon, isDrawing]);
    const dataRef = useRef<LayerData>(currentData);

    // Todo lo que leen los listeners del mapa, actualizado después de cada render (nunca durante él).
    const stateRef = useRef({ map, isDrawing, draft, polygon, addPoint, finish, cancel });
    useEffect(() => {
        stateRef.current = { map, isDrawing, draft, polygon, addPoint, finish, cancel };
        dataRef.current = currentData;
    });

    const paint = useCallback((data: LayerData) => {
        const current = stateRef.current.map;
        (current?.getSource(AREA_SOURCE) as GeoJSONSource | undefined)?.setData(data.area);
        (current?.getSource(VERTEX_SOURCE) as GeoJSONSource | undefined)?.setData(data.vertices);
    }, []);

    useEffect(() => {
        if (!map) return;
        return setupMapLayers(map, () => setupLayers(map, dataRef.current), [AREA_FILL, AREA_LINE, DRAFT_LINE, VERTEX_CIRCLES], [AREA_SOURCE, VERTEX_SOURCE]);
    }, [map]);

    // Repintar al cambiar el área, los vértices o el modo; el hover se pinta aparte, sin renders de React.
    useEffect(() => {
        hoverRef.current = null;
        paint(currentData);
    }, [currentData, paint]);

    useEffect(() => {
        if (!map) return;

        const onClick = (event: MapMouseEvent) => {
            const current = stateRef.current;
            if (!current.isDrawing) return;
            if (current.draft.length >= 3) {
                const [lng, lat] = current.draft[0];
                const first = map.project([lng, lat]);
                if (Math.hypot(event.point.x - first.x, event.point.y - first.y) <= CLOSE_HIT_RADIUS) {
                    current.finish();
                    return;
                }
            }
            current.addPoint([event.lngLat.lng, event.lngLat.lat]);
        };

        const onDoubleClick = (event: MapMouseEvent) => {
            if (!stateRef.current.isDrawing) return;
            // El doble clic de MapLibre acerca el mapa; aquí significa "terminar el polígono".
            event.preventDefault();
            stateRef.current.finish();
        };

        const onMouseMove = (event: MapMouseEvent) => {
            if (!stateRef.current.isDrawing || !stateRef.current.draft.length) return;
            const hover: Position = [event.lngLat.lng, event.lngLat.lat];
            hoverRef.current = hover;
            dataRef.current = {
                ...dataRef.current,
                area: buildArea(stateRef.current.draft, stateRef.current.polygon, hover, true),
            };
            paint(dataRef.current);
        };

        const onKeyDown = (event: KeyboardEvent) => {
            const current = stateRef.current;
            if (!current.isDrawing || event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement) return;
            if (event.key === "Enter") {
                event.preventDefault();
                current.finish();
            } else if (event.key === "Escape") {
                event.preventDefault();
                current.cancel();
            }
        };

        map.on("click", onClick);
        map.on("dblclick", onDoubleClick);
        map.on("mousemove", onMouseMove);
        document.addEventListener("keydown", onKeyDown);
        return () => {
            map.off("click", onClick);
            map.off("dblclick", onDoubleClick);
            map.off("mousemove", onMouseMove);
            document.removeEventListener("keydown", onKeyDown);
        };
    }, [map, paint]);

    useEffect(() => {
        if (!map || !isDrawing) return;
        map.doubleClickZoom.disable();
        map.getCanvas().style.cursor = "crosshair";
        return () => {
            map.doubleClickZoom.enable();
            map.getCanvas().style.cursor = "";
        };
    }, [map, isDrawing]);

    return null;
}
