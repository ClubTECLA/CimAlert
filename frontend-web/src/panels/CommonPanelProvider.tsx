import { useCallback, useRef, useState, type ReactNode } from "react";
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
    // Guard de salida que registra la vista con borrador. Todas las navegaciones pasan por él, vengan de openPanel,
    // de backView, de closePanel o de una acción como cerrar sesión que envuelva requestLeave.
    const leaveGuardRef = useRef<((action: () => void) => void) | null>(null);

    const registerLeaveGuard = useCallback((guard: ((action: () => void) => void) | null) => {
        leaveGuardRef.current = guard;
    }, []);

    const requestLeave = useCallback((action: () => void) => {
        const guard = leaveGuardRef.current;
        if (!guard) {
            action();
            return;
        }
        guard(() => {
            // Aprobado: el guard se agota para que la acción de navegación no vuelva a pedir confirmación.
            leaveGuardRef.current = null;
            action();
        });
    }, []);

    // El Provider adapta la API pública al router puro; las decisiones de historial viven en panelRouter.
    // Callbacks estables: varios efectos de las vistas los tienen como dependencia y una identidad nueva en cada render
    // provoca que se vuelvan a suscribir (y, si el efecto cierra el panel, un bucle de actualizaciones).
    const openPanel: OpenPanel = useCallback((view, ...args) => {
        const [props, options] = args;
        const entry = { view, props: props ?? {} } as PanelEntry;
        requestLeave(() => setRouterState((state) => navigateToPanel(state, entry, options)));
    }, [requestLeave]);

    const backView = useCallback(() => requestLeave(() => setRouterState((state) => navigateBack(state))), [requestLeave]);
    const closePanel = useCallback(() => requestLeave(() => setRouterState((state) => resetPanel(state))), [requestLeave]);

    return (
        <CommonPanelContext.Provider
            value={{
                currentPanelView: routerState.current?.view ?? null,
                currentPanel: routerState.current,
                openPanel,
                backView,
                hasPrevView: hasPreviousPanel(routerState),
                closePanel,
                registerLeaveGuard,
                requestLeave,
            }}
        >
            {/* El chrome se monta en main.tsx (PanelChromeMount), dentro de todos los providers de vistas. */}
            {children}
        </CommonPanelContext.Provider>
    );
}
