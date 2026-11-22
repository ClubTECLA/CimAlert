import type { Polygon, Position } from "geojson";

// Geometría pura para filtrar por área. Sin DOM ni dependencias del mapa: se prueba con vitest y la usan tanto
// reportes como centros. Las coordenadas GeoJSON van siempre [longitud, latitud].

// Tolerancia para considerar que un punto cae sobre un segmento (grados; ~1e-12 es del orden de micrómetros).
const EPSILON = 1e-12;

function isOnSegment(point: Position, a: Position, b: Position): boolean {
    const [x, y] = point;
    const [ax, ay] = a;
    const [bx, by] = b;
    const cross = (x - ax) * (by - ay) - (y - ay) * (bx - ax);
    if (Math.abs(cross) > EPSILON) return false;
    return x >= Math.min(ax, bx) - EPSILON
        && x <= Math.max(ax, bx) + EPSILON
        && y >= Math.min(ay, by) - EPSILON
        && y <= Math.max(ay, by) + EPSILON;
}

/** Ray casting sobre un anillo. El borde cuenta como dentro, para que un pin justo encima no desaparezca. */
function inRing(point: Position, ring: Position[]): boolean {
    if (ring.length < 3) return false;
    const [x, y] = point;
    let inside = false;
    for (let i = 0, j = ring.length - 1; i < ring.length; j = i++) {
        const a = ring[j];
        const b = ring[i];
        if (isOnSegment(point, a, b)) return true;
        const [ax, ay] = a;
        const [bx, by] = b;
        if ((ay > y) !== (by > y) && x < ((bx - ax) * (y - ay)) / (by - ay) + ax) {
            inside = !inside;
        }
    }
    return inside;
}

/**
 * Un punto está dentro del polígono si está en el anillo exterior y en ningún hueco.
 * Polígonos sin anillo exterior o degenerados no contienen nada.
 */
export function pointInPolygon(point: Position, polygon: Polygon): boolean {
    const [outer, ...holes] = polygon.coordinates;
    if (!outer || !inRing(point, outer)) return false;
    return !holes.some((hole) => inRing(point, hole));
}

/**
 * Vértices sueltos del dibujo → anillo GeoJSON cerrado. Quita repetidos consecutivos (el doble clic dispara dos clics)
 * y cierra con el primero. Devuelve null si no hay bastante para un área.
 */
export function polygonFromPoints(points: Position[]): Polygon | null {
    const unique: Position[] = [];
    for (const point of points) {
        const last = unique[unique.length - 1];
        if (!last || last[0] !== point[0] || last[1] !== point[1]) unique.push(point);
    }
    if (unique.length > 1) {
        const first = unique[0];
        const last = unique[unique.length - 1];
        if (first[0] === last[0] && first[1] === last[1]) unique.pop();
    }
    if (unique.length < 3) return null;
    return { type: "Polygon", coordinates: [[...unique, unique[0]]] };
}
