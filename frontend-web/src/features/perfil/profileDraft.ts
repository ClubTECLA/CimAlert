// Lógica pura del panel "Mi perfil": el borrador vive en el componente y solo se compara con lo guardado aquí.
// Sin React ni DOM, para poder probarla con vitest en el entorno por defecto (Node).

import type {
    AreaFilterPreferencesType,
    GeneralPreferencesType,
    SecurityPreferencesType,
} from "@/types/localStorage-types";

export type ProfileForm = {
    username: string;
    email: string;
}

export type ProfileDraft = {
    form: ProfileForm;
    general: GeneralPreferencesType;
    security: SecurityPreferencesType;
    area: AreaFilterPreferencesType;
}

/** Estado guardado del que parte el borrador: la sesión más las tres claves de localStorage. */
export type ProfileBaseline = Omit<ProfileDraft, "form"> & {
    user: Pick<ProfileForm, "username" | "email">;
}

/** Copia defensiva: editar el borrador no debe alterar nunca el estado guardado. */
export function buildProfileDraft(baseline: ProfileBaseline): ProfileDraft {
    return {
        form: {
            username: baseline.user.username,
            email: baseline.user.email ?? "",
        },
        general: { ...baseline.general },
        security: { ...baseline.security },
        area: { ...baseline.area },
    };
}

export function isProfileDirty(draft: ProfileDraft, saved: ProfileDraft): boolean {
    return draft.form.username !== saved.form.username
        || draft.form.email !== saved.form.email
        || draft.general.dark_mode !== saved.general.dark_mode
        || draft.general.show_location_in_searchBar !== saved.general.show_location_in_searchBar
        || draft.security.remember_in_this_device !== saved.security.remember_in_this_device
        || draft.security.two_steps_verification !== saved.security.two_steps_verification
        || draft.area.mode !== saved.area.mode
        || draft.area.threshold !== saved.area.threshold;
}
