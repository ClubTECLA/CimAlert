import { describe, expect, it } from "vitest";
import { buildProfileDraft, isProfileDirty, type ProfileBaseline, type ProfileDraft } from "./profileDraft";

const baseline: ProfileBaseline = {
    user: { username: "admin", email: "admin@example.com" },
    general: { dark_mode: false, show_location_in_searchBar: false },
    security: { remember_in_this_device: false, two_steps_verification: true },
    area: { mode: "backend", threshold: 500 },
};

function saved(): ProfileDraft {
    return buildProfileDraft(baseline);
}

describe("borrador de Mi perfil", () => {
    it("parte del estado guardado sin compartir referencias", () => {
        const draft = saved();
        expect(draft).toEqual(saved());
        draft.general.dark_mode = true;
        draft.form.username = "otro";
        expect(baseline.general.dark_mode).toBe(false);
        expect(saved().form.username).toBe("admin");
    });

    it("normaliza un correo ausente a cadena vacía", () => {
        const draft = buildProfileDraft({
            ...baseline,
            user: { username: "admin", email: undefined as unknown as string },
        });
        expect(draft.form.email).toBe("");
    });

    it("no está sucio cuando nada cambió", () => {
        expect(isProfileDirty(saved(), saved())).toBe(false);
    });

    it("detecta cambios en el formulario", () => {
        const draft = saved();
        draft.form.username = "otro";
        expect(isProfileDirty(draft, saved())).toBe(true);

        const emailDraft = saved();
        emailDraft.form.email = "otro@example.com";
        expect(isProfileDirty(emailDraft, saved())).toBe(true);
    });

    it("detecta cambios en cada grupo de preferencias", () => {
        const general = saved();
        general.general.show_location_in_searchBar = true;
        expect(isProfileDirty(general, saved())).toBe(true);

        const security = saved();
        security.security.two_steps_verification = false;
        expect(isProfileDirty(security, saved())).toBe(true);

        const mode = saved();
        mode.area.mode = "frontend";
        expect(isProfileDirty(mode, saved())).toBe(true);

        const threshold = saved();
        threshold.area.threshold = 25;
        expect(isProfileDirty(threshold, saved())).toBe(true);
    });

    it("restaurar el borrador con lo guardado lo deja limpio", () => {
        const draft = saved();
        draft.form.username = "otro";
        draft.area.mode = "auto";
        expect(isProfileDirty(draft, saved())).toBe(true);

        const restored = buildProfileDraft(baseline);
        expect(isProfileDirty(restored, saved())).toBe(false);
    });
});
