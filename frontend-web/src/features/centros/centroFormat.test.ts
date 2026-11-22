import { describe, expect, it } from "vitest";
import { DEFAULT_CENTRO_FILTERS, estaOcupado, filterCentros, lugaresDisponibles, lugaresTexto } from "./centroFormat";
import type { ApiCentro, ApiTipoCentro } from "@/types/api-types";

const centro = (id: string, tipo_centro: number, capacidad: number | null, cantidad: number | null) =>
    ({ id, tipo_centro, capacidad, cantidad }) as ApiCentro;

const refugios = { id: 1, nombre: "Refugios" } as ApiTipoCentro;
const salud = { id: 2, nombre: "Salud" } as ApiTipoCentro;
const tipos = new Map([[1, refugios], [2, salud]]);

const centros = [
    centro("refugio-libre", 1, 50, 20),
    centro("refugio-lleno", 1, 30, 30),
    centro("hospital", 2, null, null),
    centro("hospital-lleno", 2, 10, 12),
];
const ids = (list: ApiCentro[]) => list.map((item) => item.id);

describe("lugaresDisponibles", () => {
    it("es la capacidad menos la gente que hay, sin bajar de 0", () => {
        expect(lugaresDisponibles({ capacidad: 50, cantidad: 20 })).toBe(30);
        expect(lugaresDisponibles({ capacidad: 10, cantidad: 12 })).toBe(0);
    });

    it("sin capacidad o sin cantidad no se puede saber", () => {
        expect(lugaresDisponibles({ capacidad: null, cantidad: 3 })).toBeNull();
        expect(lugaresDisponibles({ capacidad: 40, cantidad: null })).toBeNull();
    });
});

describe("estaOcupado", () => {
    it("un refugio, solo sin lugares libres; sin capacidad registrada cuenta como disponible", () => {
        expect(estaOcupado({ capacidad: 30, cantidad: 30 }, refugios)).toBe(true);
        expect(estaOcupado({ capacidad: 30, cantidad: 29 }, refugios)).toBe(false);
        expect(estaOcupado({ capacidad: null, cantidad: null }, refugios)).toBe(false);
    });

    it("los demás tipos nunca están ocupados, aunque tengan los lugares llenos", () => {
        expect(estaOcupado({ capacidad: 10, cantidad: 12 }, salud)).toBe(false);
        expect(estaOcupado({ capacidad: 10, cantidad: 12 })).toBe(false);
    });
});

describe("filterCentros", () => {
    it("de entrada (activos) muestra todo menos los refugios llenos", () => {
        expect(ids(filterCentros(centros, DEFAULT_CENTRO_FILTERS, tipos))).toEqual(["refugio-libre", "hospital", "hospital-lleno"]);
    });

    it("ocupados son solo los refugios llenos; todos, todo; y oculta los tipos apagados", () => {
        expect(ids(filterCentros(centros, { tab: "ocupados", hiddenTipos: [] }, tipos))).toEqual(["refugio-lleno"]);
        expect(ids(filterCentros(centros, { tab: "todos", hiddenTipos: [] }, tipos))).toEqual(ids(centros));
        expect(ids(filterCentros(centros, { tab: "todos", hiddenTipos: [2] }, tipos))).toEqual(["refugio-libre", "refugio-lleno"]);
    });
});

describe("lugaresTexto", () => {
    it("usa el singular con un lugar", () => {
        expect(lugaresTexto(1)).toBe("1 lugar disponible");
        expect(lugaresTexto(0)).toBe("0 lugares disponibles");
    });
});
