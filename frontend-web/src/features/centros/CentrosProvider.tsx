import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type { Polygon } from "geojson";
import { useAuth } from "@/features/sesion/useAuth";
import { areaFilterStore } from "@/features/areas/areaFilterStore";
import { LOCAL_STORAGE_CHANGE_EVENT, type LocalStorageChangeDetail } from "@/lib/useLocalStorage";
import { AREA_FILTER_STORAGE_KEY } from "@/types/localStorage-types";
import { buscarArea, invalidarBuscarArea } from "@/features/areas/areas.services";
import type { ApiCentro, ApiCrearCentroRequest, ApiTipoCentro } from "@/types/api-types";
import { crearCentro, listarCentros, listarTiposCentro } from "./centros.services";
import { DEFAULT_CENTRO_FILTERS, filterCentros, type CentroFilters } from "./centroFormat";
import { CentrosContext, type CentrosContextType } from "./CentrosContext.ts";

// Datos de centros compartidos por el panel "Centros", su detalle, "Nuevo centro" y la capa del mapa: una sola
// descarga y todos se actualizan a la vez. Sin sondeo, a diferencia de los reportes: los centros cambian poco y
// solo desde aquí; se recargan al iniciar sesión y al crear uno.

// Mientras no llegan los tipos se expone un Map vacío y estable.
const NO_TIPOS = new Map<number, ApiTipoCentro>();

export function CentrosProvider({ children }: { children: ReactNode }) {
    const { isAuthenticated } = useAuth();

    const [centros, setCentros] = useState<ApiCentro[] | null>(null);
    // null hasta que el catálogo llega bien por primera vez.
    const [tipos, setTipos] = useState<Map<number, ApiTipoCentro> | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [filters, setFilters] = useState<CentroFilters>(DEFAULT_CENTRO_FILTERS);

    // Igual que ReportesProvider: al cerrar sesión se descartan las respuestas que lleguen tarde, y entre cargas que
    // coinciden solo se aplica la última.
    const sessionRef = useRef(0);
    const requestRef = useRef(0);
    // Igual que en ReportesProvider: refresh no cambia de identidad al dibujar, para no reiniciar cargas ni efectos.
    const polygonRef = useRef<Polygon | null>(filters.polygon ?? null);

    const refresh = useCallback(async () => {
        const session = sessionRef.current;
        const request = ++requestRef.current;
        const polygon = polygonRef.current;
        // Misma decisión que ReportesProvider, leída del store compartido; buscarArea deduplica la petición
        // cuando ambos providers la piden a la vez.
        const enBackend = polygon !== null && areaFilterStore.resolveMode() === "backend";
        const [centroResult, tipoResult] = await Promise.allSettled([
            enBackend && polygon ? buscarArea(polygon).then((respuesta) => respuesta.centros) : listarCentros(),
            listarTiposCentro(),
        ]);
        if (session !== sessionRef.current || request !== requestRef.current) return;

        // Cada lista por separado: si falla una, la otra se aplica y lo que falla conserva lo último que se cargó bien.
        if (centroResult.status === "fulfilled") setCentros(centroResult.value);
        if (tipoResult.status === "fulfilled") setTipos(new Map(tipoResult.value.map((tipo) => [tipo.id, tipo])));

        const failed = [centroResult, tipoResult].find((result) => result.status === "rejected");
        setError(failed
            ? (failed.reason instanceof Error ? failed.reason.message : "No se pudieron cargar los centros.")
            : null);
    }, []);

    const crear = useCallback(async (nuevo: ApiCrearCentroRequest) => {
        const session = sessionRef.current;
        const centro = await crearCentro(nuevo);
        // Si se cerró sesión mientras tanto, el centro ya existe en el servidor pero no se toca el estado (ya vacío):
        // se devuelve igual, como cualquier alta que terminó bien, sin añadirlo ni recargar.
        if (session !== sessionRef.current) return centro;
        // Se añade ya el que devolvió el POST: si la recarga falla, su detalle (que se abre al publicar) lo encuentra
        // igual. Si la recarga sale bien, la lista del servidor lo sustituye.
        setCentros((prev) => [...(prev ?? []).filter((item) => item.id !== centro.id), centro]);
        // Sin filtros, para que la lista y el mapa muestren el centro nuevo, salvo el área dibujada: crear un centro
        // no debe quitar el filtro que el usuario eligió (se quita con "Limpiar selección" o restableciendo filtros).
        setFilters((prev) => ({ ...DEFAULT_CENTRO_FILTERS, polygon: prev.polygon }));
        // La recarga del alta necesita datos posteriores al POST: si había una búsqueda por área en vuelo, reutilizarla
        // dejaría fuera el centro recién creado. Solo se invalida aquí; las lecturas normales siguen compartiendo resultados.
        invalidarBuscarArea();
        await refresh();
        return centro;
    }, [refresh]);

    useEffect(() => {
        if (!isAuthenticated) {
            sessionRef.current++;
            return;
        }
        // Carga inicial fuera del cuerpo del efecto (refresh actualiza el estado al llegar la respuesta).
        const initialLoadId = setTimeout(() => { void refresh() }, 0);
        return () => {
            clearTimeout(initialLoadId);
            // Al cerrar sesión no deben quedar datos del usuario anterior.
            setCentros(null);
            setTipos(null);
            setError(null);
        };
    }, [isAuthenticated, refresh]);

    // Al dibujar, limpiar o restablecer el área cambia la fuente de datos; los demás filtros siguen siendo del cliente.
    useEffect(() => {
        const polygon = filters.polygon ?? null;
        if (polygonRef.current === polygon) return; // montaje inicial: ya lo carga el efecto de la sesión
        polygonRef.current = polygon;
        if (isAuthenticated) void refresh();
    }, [filters.polygon, isAuthenticated, refresh]);

    useEffect(() => {
        const onChange = (event: Event) => {
            const detail = (event as CustomEvent<LocalStorageChangeDetail>).detail;
            if (detail?.key === AREA_FILTER_STORAGE_KEY && isAuthenticated) void refresh();
        };
        // El evento propio cubre esta pestaña; el nativo `storage` cubre los cambios de preferencias en otras pestañas.
        const onStorage = (event: StorageEvent) => {
            if (event.key === AREA_FILTER_STORAGE_KEY && isAuthenticated) void refresh();
        };
        window.addEventListener(LOCAL_STORAGE_CHANGE_EVENT, onChange);
        window.addEventListener("storage", onStorage);
        return () => {
            window.removeEventListener(LOCAL_STORAGE_CHANGE_EVENT, onChange);
            window.removeEventListener("storage", onStorage);
        };
    }, [isAuthenticated, refresh]);

    // Sin el catálogo no se sabe qué centros son refugios (ni si están ocupados): hasta que llega, los centros siguen
    // "cargando" (o en error) para todos, en vez de clasificarlos mal.
    const listos = tipos ? centros : null;
    const filteredCentros = useMemo(() => tipos ? filterCentros(listos ?? [], filters, tipos) : [], [listos, filters, tipos]);

    const value = useMemo<CentrosContextType>(() => ({
        centros: listos,
        tipos: tipos ?? NO_TIPOS,
        error,
        filteredCentros,
        filters,
        setFilters,
        refresh,
        crear,
    }), [listos, tipos, error, filteredCentros, filters, refresh, crear]);

    return (
        <CentrosContext.Provider value={value}>
            {children}
        </CentrosContext.Provider>
    );
}
