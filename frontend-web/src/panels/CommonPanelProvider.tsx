import { useCallback, useState, type ReactNode } from "react";
import { validateChildren } from "@/lib/validateChildren";
import {
    backPanel as navigateBack,
    closePanel as resetPanel,
    createPanelRouterState,
    hasPreviousPanel,
    openPanel as navigateToPanel,
} from "./panelRouter";
import { CommonPanelContext } from "./CommonPanelContext.ts";
import type { OpenPanel, PanelEntry } from "./panelTypes";

export function CommonPanelProvider({ children }: { children: ReactNode }) {
    validateChildren(children, "CommonPanelProvider");

    const [routerState, setRouterState] = useState(createPanelRouterState);

    // El Provider adapta la API pública al router puro; las decisiones de historial viven en panelRouter.
    // Callbacks estables: varios efectos de las vistas los tienen como dependencia y una identidad nueva en cada render
    // provoca que se vuelvan a suscribir (y, si el efecto cierra el panel, un bucle de actualizaciones).
    const openPanel: OpenPanel = useCallback((view, ...args) => {
        const [props, options] = args;
        const entry = { view, props: props ?? {} } as PanelEntry;
        setRouterState((state) => navigateToPanel(state, entry, options));
    }, []);

    const backView = useCallback(() => setRouterState((state) => navigateBack(state)), []);
    const closePanel = useCallback(() => setRouterState((state) => resetPanel(state)), []);

    return (
        <CommonPanelContext.Provider
            value={{
                currentPanelView: routerState.current?.view ?? null,
                currentPanel: routerState.current,
                openPanel,
                backView,
                hasPrevView: hasPreviousPanel(routerState),
                closePanel,
            }}
        >
            {/* El chrome se monta en main.tsx (PanelChromeMount), dentro de todos los providers de vistas. */}
            {children}
        </CommonPanelContext.Provider>
    );
}
