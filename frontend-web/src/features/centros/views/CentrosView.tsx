import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { LuPlus } from "react-icons/lu";
import ColSection from "@/components/layout/ColSection";
import CategoryChip from "@/components/ui/CategoryChip";
import Button from "@/components/ui/Button";
import { SegmentedTabs, TabItem } from "@/components/ui/SegmentedTabs";
import { useAuth } from "@/features/sesion/useAuth";
import { canManageCentros } from "@/features/sesion/users";
import { useCommonPanel } from "@/panels/useCommonPanel";
import { useCentros } from "@/features/centros/useCentros";
import { toggled } from "@/features/reportes/reportFilters";
import { estaOcupado, filterCentros, type CentroFilters, type CentroTab } from "@/features/centros/centroFormat";
import CentroRow from "@/features/centros/components/CentroRow";
import { esRefugio, tipoStyle } from "@/features/centros/tiposCentro";

// Filas que añade cada "Cargar N más", igual que la lista de incidentes. El backend no pagina: se pagina aquí.
const PAGE_SIZE = 5;

const noteStyle = "py-6 text-center text-sm text-ink-muted";

// "Lista los mismos centros que pinta el mapa (el filtro de CentrosContext):
// las pestañas y los chips de tipo también cambian el mapa.
export default function CentrosView() {
    const { user } = useAuth();
    const { openPanel } = useCommonPanel();
    const { centros, tipos, error, filteredCentros: filtered, filters, setFilters } = useCentros();
    // La primera tanda son las filas que caben enteras en el panel (fitCount); "Cargar más" suma tandas (extra).
    // fitCount 0: ni la primera cabe (panel muy bajo), así que se muestra igual y la lista pasa a tener scroll.
    const [fitCount, setFitCount] = useState(PAGE_SIZE);
    const [extra, setExtra] = useState(0);
    const listRef = useRef<HTMLDivElement>(null);
    const [listHeight, setListHeight] = useState(0);

    const all = centros ?? [];
    const refugiosDisponibles = all.filter((centro) => {
        const tipo = tipos.get(centro.tipo_centro);
        return esRefugio(tipo) && !estaOcupado(centro, tipo);
    }).length;
    const shown = filtered.slice(0, Math.max(fitCount, 1) + extra);
    const remaining = filtered.length - shown.length;
    const scrolls = extra > 0 || fitCount === 0;
    // Mientras no se carga más, la siguiente fila se pinta invisible para medir si cabe entera.
    const probe = scrolls ? undefined : filtered[shown.length];

    // Ajusta la primera tanda al alto del panel antes de pintar: si la sonda cabe, se muestra (y se mide la siguiente);
    // si la última fila ya no cabe (el panel se encogió), se quita. Las filas no miden todas lo mismo (los refugios
    // llevan una línea más), por eso se mide cada una y no se divide el alto.
    useLayoutEffect(() => {
        const list = listRef.current;
        if (!list || extra > 0) return;
        const bottom = list.getBoundingClientRect().bottom;
        // Con scrollTop, para medir igual aunque la fila única se haya desplazado.
        const fits = (element: Element | null | undefined) =>
            !!element && element.getBoundingClientRect().bottom + list.scrollTop <= bottom;
        const rows = list.querySelectorAll(":scope > button");
        if (rows.length === 0) return;
        if (!fits(rows[rows.length - 1])) setFitCount(rows.length - 1);
        else if (fitCount === 0) setFitCount(1);
        else if (fits(list.querySelector(":scope > [data-probe]"))) setFitCount(rows.length + 1);
    }, [extra, fitCount, filtered, listHeight]);

    // Al cambiar el alto del panel (ventana, teléfono girado) se vuelve a medir.
    useEffect(() => {
        const list = listRef.current;
        if (!list) return;
        const observer = new ResizeObserver(() => setListHeight(list.clientHeight));
        observer.observe(list);
        return () => observer.disconnect();
    }, []);

    // Cambiar el filtro vuelve a la primera tanda.
    const update = (changes: Partial<CentroFilters>) => { setFilters({ ...filters, ...changes }); setExtra(0) };

    const tabs: { key: CentroTab; label: string }[] = [
        // El contador respeta los tipos elegidos, igual que la lista.
        { key: "activos", label: centros ? `Activos · ${filterCentros(all, { ...filters, tab: "activos" }, tipos).length}` : "Activos" },
        { key: "ocupados", label: "Ocupados" },
        { key: "todos", label: "Todos" },
    ];

    // Solo el administrador puede crear centros (POST /centro).
    const newButton = canManageCentros(user?.rol_name) &&
        <Button variant="mint" size="lg" icon={<LuPlus />} title="Agregar" onClick={() => openPanel("Nuevo Centro")} />;

    return (
        <ColSection
            title="Centros de ayuda"
            subtitle={centros ? `${all.length} centros · ${refugiosDisponibles} refugios libres` : "Cargando…"}
            headerActions={newButton}
            arrangementSubsections="toUp"
            className="min-h-0 overflow-visible"
        >
            {/* Mismo ancho fijo que el resto de paneles: con solo un mínimo, el panel (w-fit) crecería hasta el nombre
                más largo en vez de recortarlo con "…". En teléfono, el interior del panel (100vw menos left-2, right-2 y
                p-3 de PanelChrome) y no w-full: dentro del w-fit de ColSection, w-full se ajusta al contenido y las
                filas cortas de centros dejaban un hueco a la derecha. */}
            <div className="flex min-h-0 w-[calc(100vw-2.5rem)] flex-1 flex-col gap-3.5 font-ui sm:w-[max(25rem,calc(25vw-2rem))]">
                <SegmentedTabs
                    format="row"
                    activeTab={tabs.findIndex(({ key }) => key === filters.tab)}
                    onChange={(index) => update({ tab: tabs[index].key })}
                >
                    {tabs.map(({ key, label }) => <TabItem key={key} tabTitle={label} />)}
                </SegmentedTabs>

                {tipos.size > 0 &&
                    <div className="flex flex-wrap gap-2">
                        {/* En "Ocupados" solo puede haber refugios: los chips de otros tipos no filtrarían nada. */}
                        {[...tipos.values()].filter((tipo) => filters.tab !== "ocupados" || esRefugio(tipo)).map((tipo) =>
                            <CategoryChip
                                key={tipo.id}
                                label={tipo.nombre}
                                color={tipoStyle(tipo).color}
                                isSelected={!filters.hiddenTipos.includes(tipo.id)}
                                onPress={() => update({ hiddenTipos: toggled(filters.hiddenTipos, tipo.id) })}
                            />
                        )}
                    </div>
                }

                {/* El scroll se extiende sobre el relleno del panel para quedar pegado a su borde derecho. Por eso
                    ColSection va con overflow-visible: con scroll propio, ese sobrante le daría una barra horizontal.
                    Sin scroll hasta "Cargar más" (o si ni una fila cabe): las filas visibles caben enteras y la sonda que
                    sobresale se recorta. */}
                <div
                    ref={listRef}
                    className={`-mr-3 flex min-h-0 flex-1 flex-col gap-2 p-0.5 pr-3.5 sm:-mr-4 sm:pr-4.5 ${scrolls ? "overflow-y-auto" : "overflow-hidden"}`}
                >
                    {error && <span className={noteStyle}>{error}</span>}
                    {!error && !centros && <span className={noteStyle}>Cargando centros…</span>}
                    {centros && filtered.length === 0 &&
                        <span className={noteStyle}>
                            {all.length === 0 ? "Todavía no hay centros." : "No hay centros con estos filtros."}
                        </span>
                    }
                    {shown.map((centro) =>
                        <CentroRow
                            key={centro.id}
                            centro={centro}
                            tipo={tipos.get(centro.tipo_centro)}
                            onClick={() => openPanel("Detalle Centro", { centroId: centro.id })}
                        />
                    )}
                    {/* invisible (y no hidden): ocupa su alto para medirlo, pero no se ve, no se enfoca ni se lee. */}
                    {probe &&
                        <div data-probe className="invisible shrink-0" aria-hidden>
                            <CentroRow centro={probe} tipo={tipos.get(probe.tipo_centro)} onClick={() => undefined} />
                        </div>
                    }
                </div>

                {remaining > 0 &&
                    <button
                        type="button"
                        onClick={() => setExtra((prev) => prev + PAGE_SIZE)}
                        className="w-full shrink-0 rounded-full bg-canvas py-2 text-sm font-semibold text-ink transition-colors hover:bg-line-soft"
                    >
                        Cargar {Math.min(PAGE_SIZE, remaining)} más
                    </button>
                }
            </div>
        </ColSection>
    );
}
