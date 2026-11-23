import type { Polygon, Position } from "geojson";

// Geometría pura para filtrar por área. Sin DOM ni dependencias del mapa: se prueba con vitest y la usan tanto
// reportes como centros. Las coordenadas GeoJSON van siempre [longitud, latitud].

// Tolerancia para considerar que un punto cae sobre un segmento. Tras la proyección tangente las unidades son
// radianes, así que 1e-12 sigue siendo del orden de micrómetros.
const EPSILON = 1e-12;
const RADIANES = Math.PI / 180;
type Vector3 = [number, number, number];

function vector(point: Position): Vector3 {
    const [lng, lat] = point;
    const longitud = lng * RADIANES;
    const latitud = lat * RADIANES;
    const cosLatitud = Math.cos(latitud);
    return [cosLatitud * Math.cos(longitud), cosLatitud * Math.sin(longitud), Math.sin(latitud)];
}

function productoEscalar(a: Vector3, b: Vector3): number {
    return a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
}

/**
 * Proyección gnomónica centrada en el punto consultado: los círculos máximos se vuelven rectas, así que el mismo
 * ray-casting de abajo respeta los bordes geodésicos que usa PostGIS sobre `geography`. El punto consultado queda
 * en (0, 0). Devuelve null para vértices a más de 90° (la proyección no está definida).
 */
function proyectar(vertice: Position, centro: Position): Position | null {
    const v = vector(vertice);
    const p = vector(centro);
    const longitud = centro[0] * RADIANES;
    const latitud = centro[1] * RADIANES;
    const este: Vector3 = [-Math.sin(longitud), Math.cos(longitud), 0];
    const norte: Vector3 = [-Math.sin(latitud) * Math.cos(longitud), -Math.sin(latitud) * Math.sin(longitud), Math.cos(latitud)];
    const z = productoEscalar(v, p);
    if (z <= 0) return null;
    return [productoEscalar(v, este) / z, productoEscalar(v, norte) / z];
}

function anilloProyectado(anillo: Position[], centro: Position): Position[] | null {
    const proyectados: Position[] = [];
    for (const vertice of anillo) {
        const punto = proyectar(vertice, centro);
        if (!punto) return null;
        proyectados.push(punto);
    }
    return proyectados;
}

function isOnSegment(point: Position, a: Position, b: Position): boolean {
    const [x, y] = point;
    const [ax, ay] = a;
    const [bx, by] = b;
    const dx = bx - ax;
    const dy = by - ay;
    const longitud = Math.hypot(dx, dy);
    // La tolerancia es una distancia perpendicular, no un producto cruzado sin normalizar: si no, un segmento corto
    // admitiría puntos fuera de él. Vértices repetidos (longitud 0) solo cubren su propia posición.
    if (longitud === 0) return Math.hypot(x - ax, y - ay) <= EPSILON;
    const cross = (x - ax) * dy - (y - ay) * dx;
    if (Math.abs(cross) > EPSILON * longitud) return false;
    return x >= Math.min(ax, bx) - EPSILON
        && x <= Math.max(ax, bx) + EPSILON
        && y >= Math.min(ay, by) - EPSILON
        && y <= Math.max(ay, by) + EPSILON;
}

/** Ray casting sobre un anillo. `bordeDentro` distingue el exterior (sí) del borde de un hueco (no). */
function inRing(point: Position, ring: Position[], bordeDentro = true): boolean {
    if (ring.length < 3) return false;
    const [x, y] = point;
    let inside = false;
    for (let i = 0, j = ring.length - 1; i < ring.length; j = i++) {
        const a = ring[j];
        const b = ring[i];
        if (isOnSegment(point, a, b)) return bordeDentro;
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
    if (!outer) return false;
    const exterior = anilloProyectado(outer, point);
    if (!exterior || !inRing([0, 0], exterior)) return false;
    for (const hole of holes) {
        const proyectado = anilloProyectado(hole, point);
        // ST_Covers incluye los bordes del polígono, también los de los huecos: solo se excluye su interior estricto.
        if (!proyectado || inRing([0, 0], proyectado, false)) return false;
    }
    return true;
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
    // Shoelace sin partir por dos, igual que el backend: tres vértices alineados no encierran nada.
    let area = 0;
    for (let i = 0; i < unique.length; i++) {
        const [x1, y1] = unique[i];
        const [x2, y2] = unique[(i + 1) % unique.length];
        area += x1 * y2 - x2 * y1;
    }
    if (Math.abs(area) < 1e-12) return null;
    return { type: "Polygon", coordinates: [[...unique, unique[0]]] };
}
