import type { Polygon } from "geojson";
import { describe, expect, it } from "vitest";
import { pointInPolygon, polygonFromPoints } from "./geo";

const square = (x0: number, y0: number, x1: number, y1: number): number[][] => [
    [x0, y0], [x1, y0], [x1, y1], [x0, y1], [x0, y0],
];
const polygon = (...rings: number[][][]): Polygon => ({ type: "Polygon", coordinates: rings });

describe("pointInPolygon", () => {
    const area = polygon(square(-117.1, 32.5, -117.0, 32.6));

    it("acepta puntos interiores y rechaza los exteriores", () => {
        expect(pointInPolygon([-117.05, 32.55], area)).toBe(true);
        expect(pointInPolygon([-117.2, 32.55], area)).toBe(false);
        expect(pointInPolygon([-117.05, 32.7], area)).toBe(false);
    });

    it("el borde y los vértices cuentan como dentro", () => {
        expect(pointInPolygon([-117.1, 32.55], area)).toBe(true);
        expect(pointInPolygon([-117.0, 32.6], area)).toBe(true);
        expect(pointInPolygon([-117.1, 32.5], area)).toBe(true);
    });

    it("los bordes horizontales siguen el arco geodésico, igual que ST_Covers sobre geography", () => {
        // El borde sur se eleva ~9.9e-6° en el punto medio: un punto a 32.5 (o justo encima) queda fuera aunque
        // con arcos rectos de latitud/longitud pareciera estar dentro. El borde norte se eleva igual: el punto
        // justo por encima de 32.6 sí cae dentro del área en el servidor.
        expect(pointInPolygon([-117.05, 32.5], area)).toBe(false);
        expect(pointInPolygon([-117.05, 32.500005], area)).toBe(false);
        expect(pointInPolygon([-117.05, 32.600005], area)).toBe(true);
    });

    it("respeta cóncavos: el hueco de una L queda fuera", () => {
        const l = polygon([
            [0, 0], [4, 0], [4, 1], [1, 1], [1, 4], [0, 4], [0, 0],
        ]);
        expect(pointInPolygon([0.5, 0.5], l)).toBe(true);
        expect(pointInPolygon([2, 2], l)).toBe(false);
    });

    it("excluye los huecos pero no el exterior", () => {
        const conHueco = polygon(square(0, 0, 10, 10), square(3, 3, 7, 7));
        expect(pointInPolygon([1, 1], conHueco)).toBe(true);
        expect(pointInPolygon([5, 5], conHueco)).toBe(false);
    });

    it("el borde del hueco cuenta como parte del polígono", () => {
        const conHueco = polygon(square(0, 0, 10, 10), square(3, 3, 7, 7));
        expect(pointInPolygon([3, 3], conHueco)).toBe(true);
        expect(pointInPolygon([5, 3], conHueco)).toBe(true);
        expect(pointInPolygon([5, 5], conHueco)).toBe(false);
    });

    it("un punto cerca de un segmento corto no se toma como borde", () => {
        const triangulo = polygon([[0, 0], [0.0001, 0.0001], [0, 0.0001], [0, 0]]);
        expect(pointInPolygon([0.00006, 0.00004], triangulo)).toBe(false);
        expect(pointInPolygon([0.00005, 0.00005], triangulo)).toBe(true);
    });

    it("un polígono vacío o degenerado no contiene nada", () => {
        expect(pointInPolygon([0, 0], polygon())).toBe(false);
        expect(pointInPolygon([0, 0], polygon([[0, 0], [0, 0]]))).toBe(false);
    });
});

describe("polygonFromPoints", () => {
    it("cierra el anillo con el primer vértice", () => {
        expect(polygonFromPoints([[0, 0], [2, 0], [2, 2], [0, 2]])).toEqual(
            polygon([[0, 0], [2, 0], [2, 2], [0, 2], [0, 0]]),
        );
    });

    it("quita repetidos consecutivos y el cierre que ya traía el dibujo", () => {
        expect(polygonFromPoints([[0, 0], [2, 0], [2, 0], [2, 2], [0, 0]])).toEqual(
            polygon([[0, 0], [2, 0], [2, 2], [0, 0]]),
        );
    });

    it("con menos de tres vértices no hay área", () => {
        expect(polygonFromPoints([])).toBeNull();
        expect(polygonFromPoints([[0, 0], [1, 1]])).toBeNull();
    });

    it("tres vértices alineados no encierran área", () => {
        expect(polygonFromPoints([[0, 0], [1, 1], [2, 2]])).toBeNull();
    });
});
