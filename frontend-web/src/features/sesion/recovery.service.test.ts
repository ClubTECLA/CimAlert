import { afterEach, describe, expect, it, vi } from "vitest";
import { restablecerContrasena, solicitarRecuperacion, verificarRecuperacion } from "./recovery.service";

afterEach(() => {
    vi.unstubAllGlobals();
});

describe("recovery service", () => {
    it("solicita un PIN sin autenticación y acepta la respuesta 202 vacía", async () => {
        const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 202 }));
        vi.stubGlobal("fetch", fetchMock);

        await expect(solicitarRecuperacion({ email: "persona@example.com" })).resolves.toBeUndefined();

        const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
        expect(url).toContain("/auth/recovery");
        expect(init.method).toBe("POST");
        expect(JSON.parse(String(init.body))).toEqual({ email: "persona@example.com" });
        expect(new Headers(init.headers).has("Authorization")).toBe(false);
    });

    it("verifica el PIN y devuelve el token de reset sin autenticación", async () => {
        const fetchMock = vi.fn().mockResolvedValue(
            Response.json({ reset_token: "token-de-un-solo-uso" }, { status: 200 }),
        );
        vi.stubGlobal("fetch", fetchMock);

        await expect(verificarRecuperacion({
            email: "persona@example.com",
            pin: "123456",
        })).resolves.toEqual({ reset_token: "token-de-un-solo-uso" });

        const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
        expect(url).toContain("/auth/recovery/verify");
        expect(init.method).toBe("POST");
        expect(JSON.parse(String(init.body))).toEqual({
            email: "persona@example.com",
            pin: "123456",
        });
        expect(new Headers(init.headers).has("Authorization")).toBe(false);
    });

    it("guarda la contraseña con el token de reset en el endpoint de restablecimiento", async () => {
        const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
        vi.stubGlobal("fetch", fetchMock);

        await expect(restablecerContrasena({
            reset_token: "token-de-un-solo-uso",
            new_password: "NuevaClave123!",
        })).resolves.toBeUndefined();

        const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
        expect(url).toContain("/auth/recovery/reset");
        expect(init.method).toBe("POST");
        expect(JSON.parse(String(init.body))).toEqual({
            reset_token: "token-de-un-solo-uso",
            new_password: "NuevaClave123!",
        });
        expect(new Headers(init.headers).has("Authorization")).toBe(false);
    });
});
