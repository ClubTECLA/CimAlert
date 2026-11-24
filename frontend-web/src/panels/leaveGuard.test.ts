import { describe, expect, it } from "vitest";
import { createLeaveGuardCoordinator } from "./leaveGuard";

describe("createLeaveGuardCoordinator", () => {
    it("aprueba una vez las navegaciones anidadas de una acción aprobada", () => {
        const coordinator = createLeaveGuardCoordinator();
        const calls: string[] = [];
        let asks = 0;
        coordinator.register((proceed) => {
            asks += 1;
            proceed();
        });

        coordinator.request(() => {
            calls.push("externa");
            coordinator.request(() => calls.push("anidada"));
        });

        expect(asks).toBe(1);
        expect(calls).toEqual(["externa", "anidada"]);
    });

    it("conserva el guard tras una acción aprobada que no cambia el panel", () => {
        const coordinator = createLeaveGuardCoordinator();
        const calls: string[] = [];
        let asks = 0;
        coordinator.register((proceed) => {
            asks += 1;
            proceed();
        });

        coordinator.request(() => { calls.push("no-op"); });
        coordinator.request(() => { calls.push("después"); });

        expect(asks).toBe(2);
        expect(calls).toEqual(["no-op", "después"]);
    });

    it("solo register(null) retira el guard", () => {
        const coordinator = createLeaveGuardCoordinator();
        let asks = 0;
        coordinator.register((proceed) => {
            asks += 1;
            proceed();
        });

        coordinator.request(() => {});
        expect(asks).toBe(1);

        coordinator.register(null);
        coordinator.request(() => {});
        expect(asks).toBe(1);
    });

    it("mantiene el guard si se cancela o si la acción falla", () => {
        const coordinator = createLeaveGuardCoordinator();
        let asks = 0;
        coordinator.register(() => {
            asks += 1;
            // Cancelar: no se llama a proceed().
        });

        coordinator.request(() => { throw new Error("no debería ejecutarse"); });
        expect(asks).toBe(1);

        coordinator.register((proceed) => {
            asks += 1;
            proceed();
        });
        expect(() => coordinator.request(() => { throw new Error("falló"); })).toThrow("falló");
        coordinator.request(() => {});
        expect(asks).toBe(3);
    });
});
