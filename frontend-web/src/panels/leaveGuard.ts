export type LeaveGuard = (action: () => void) => void;
export type LeaveAction = () => void;

export interface LeaveGuardCoordinator {
    /** Registra el guard de la vista; null solo cuando su dueño se desmonta. */
    register(guard: LeaveGuard | null): void;
    /** Pide permiso para navegar, o ejecuta la acción directamente si ya fue aprobada. */
    request(action: LeaveAction): void;
}

/**
 * El guard permanece registrado hasta que la vista que lo pone se desmonta. Aprobar una acción solo activa un atajo
 * mientras esa acción se ejecuta: así una navegación anidada no vuelve a preguntar sin borrar el guard cuando la
 * navegación aprobada resulta ser un no-op (misma vista o historial vacío) y la vista sigue montada.
 */
export function createLeaveGuardCoordinator(): LeaveGuardCoordinator {
    let guard: LeaveGuard | null = null;
    let approvedDepth = 0;

    return {
        register(next) {
            guard = next;
        },
        request(action) {
            // El atajo se comprueba primero: una navegación anidada no debe preguntar ni aunque el guard cambie.
            if (approvedDepth > 0 || !guard) {
                action();
                return;
            }
            guard(() => {
                approvedDepth += 1;
                try {
                    action();
                } finally {
                    approvedDepth -= 1;
                }
            });
        },
    };
}
