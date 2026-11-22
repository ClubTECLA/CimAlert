import { useState, type ChangeEvent } from "react";
import { FiHelpCircle, FiMoon } from "react-icons/fi";
import ColSection from "@/components/layout/ColSection";
import Input from "@/components/ui/Input";
import { ListTile, OptionList } from "@/components/ui/OptionsList";
import UserCard from "@/features/perfil/components/UserCard";
import { GoShieldCheck } from "react-icons/go";
import { LuLock, LuSun } from "react-icons/lu";
import { IoMdNotificationsOutline } from "react-icons/io";
import { useAuth } from "@/features/sesion/useAuth";
import { FaUserLock } from "react-icons/fa6";
import Button from "@/components/ui/Button";
import RowSection from "@/components/layout/RowSection";
import { useCommonPanel } from "@/panels/useCommonPanel";
import { GrMapLocation } from "react-icons/gr";
import { useLocalStorage } from "@/lib/useLocalStorage";
import { useSysMessage } from "@/components/ui/SysMessageContext";
import useModals from "@/panels/useModals";
import AreaFilterPreferences from "@/features/areas/views/AreaFilterPreferences";
import { normalizeAreaPreferences } from "@/features/areas/areaPreferences";
import {
    AREA_FILTER_STORAGE_KEY,
    type GeneralPreferencesType,
    type SecurityPreferencesType,
} from "@/types/localStorage-types";
import { hasOwnKey } from "@/lib/utils";
import {
    buildProfileDraft,
    isProfileDirty,
    type ProfileDraft,
    type ProfileForm,
} from "@/features/perfil/profileDraft";
import type { SessionUser } from "@/types/api-types";

type ProfileViewProps = {
    user: SessionUser;
}

/**
 * El panel trabaja con un borrador: nada se escribe en localStorage hasta "Guardar Cambios". "Descartar" vuelve al
 * estado guardado y salir con cambios pendientes abre una confirmación con `fullFocus`. La clave del componente
 * fuerza un borrador nuevo si cambia de usuario, sin necesidad de sincronizar con un efecto.
 */
function ProfilePanel({ user }: ProfileViewProps) {
    const [generalPreferences, setGeneralPreferences] = useLocalStorage('Preferencias generales');
    const [securityPreferences, setSecurityPreferences] = useLocalStorage('Preferencias de seguridad');
    const [areaPreferences, setAreaPreferences] = useLocalStorage(AREA_FILTER_STORAGE_KEY);
    const { openPanel, backView, closePanel } = useCommonPanel();
    const { openModal } = useModals();
    const { showMessage } = useSysMessage();

    const [savedForm, setSavedForm] = useState<ProfileForm>(() => ({
        username: user.username,
        email: user.email ?? "",
    }));
    const [draft, setDraft] = useState<ProfileDraft>(() => buildProfileDraft({
        user,
        general: generalPreferences,
        security: securityPreferences,
        area: normalizeAreaPreferences(areaPreferences),
    }));

    // Las preferencias guardadas se leen del hook: si otra pestaña las cambia, la comparación sigue siendo correcta.
    const saved: ProfileDraft = {
        form: savedForm,
        general: generalPreferences,
        security: securityPreferences,
        area: normalizeAreaPreferences(areaPreferences),
    };
    const dirty = isProfileDirty(draft, saved);

    const resetDraft = () => {
        setDraft(buildProfileDraft({
            user: saved.form,
            general: saved.general,
            security: saved.security,
            area: saved.area,
        }));
    };

    const handleChangeCheckboxPreferences = (
        key: keyof GeneralPreferencesType | keyof SecurityPreferencesType,
        value: boolean,
    ) => {
        if (hasOwnKey(draft.general, key)) {
            setDraft((prev) => ({ ...prev, general: { ...prev.general, [key]: value } }));
        } else if (hasOwnKey(draft.security, key)) {
            setDraft((prev) => ({ ...prev, security: { ...prev.security, [key]: value } }));
        }
    };

    const handleChangeForm = (event: ChangeEvent<HTMLInputElement>) => {
        const field = event.target.name as keyof ProfileForm;
        setDraft((prev) => ({ ...prev, form: { ...prev.form, [field]: event.target.value } }));
    };

    const handleSave = () => {
        const savedDraft = { ...draft, area: normalizeAreaPreferences(draft.area) };
        setDraft(savedDraft);
        setSavedForm(savedDraft.form);
        setGeneralPreferences(savedDraft.general);
        setSecurityPreferences(savedDraft.security);
        setAreaPreferences(savedDraft.area);
        showMessage("Cambios guardados correctamente.", { type: "float", color: "green", title: "Mi perfil" });
    };

    /** Sale del panel solo si no hay cambios; si los hay, ofrece descartarlos desde el modal. */
    const requestLeave = (action: () => void) => {
        if (!dirty) {
            action();
            return;
        }

        openModal("Confirmar descarte de cambios", {
            onDiscard: () => {
                resetDraft();
                action();
            },
        }, { fullFocus: true });
    };

    return (
        <ColSection
            arrangementSubsections="toDown"
            title="Mi perfil"
            subtitle="Cuenta institucional - Proteccion Civil"
            className="pb-10 overflow-y-auto px-2"
            onClose={() => requestLeave(closePanel)}
            onBack={() => requestLeave(backView)}
        >
            <UserCard
                format="col"
                username={draft.form.username}
                email={draft.form.email}
                rolName={user.rol_name}
                bg_color='blue'
            />
            <Input
                format="col"
                label="Nombre visible"
                debug={false}
                value={draft.form.username}
                onChange={handleChangeForm}
                name="username"
            />
            <Input
                format="col"
                label="Correo electronico"
                debug={false}
                value={draft.form.email}
                onChange={handleChangeForm}
                name="email"
            />
            <OptionList format="col" title="Seguridad">
                <ListTile
                    tileName="Cambiar contraseña"
                    tileDescription="Actualizada hace 3 meses"
                    onClick={() => {}}
                    icon={<LuLock />}
                    disabled={true}
                />
                <ListTile
                    tileName="Verificacion en dos pasos"
                    tileDescription="Codigo por SMS al iniciar sesion"
                    type="checkbox"
                    checked={draft.security.two_steps_verification}
                    onChange={() => handleChangeCheckboxPreferences("two_steps_verification", !draft.security.two_steps_verification)}
                    icon={<GoShieldCheck />}
                />
            </OptionList>
            <OptionList format="col" title="Preferencias">
                <ListTile
                    tileName="Tema oscuro"
                    tileDescription="Usa la version oscura de consola"
                    onChange={() => handleChangeCheckboxPreferences("dark_mode", !draft.general.dark_mode)}
                    checked={draft.general.dark_mode}
                    type="checkbox"
                    icon={<FiMoon />}
                    disabled={true}
                />
                <ListTile
                    type='checkbox'
                    onChange={() => handleChangeCheckboxPreferences("show_location_in_searchBar", !draft.general.show_location_in_searchBar)}
                    checked={draft.general.show_location_in_searchBar}
                    tileName="Mostrar ubicacion al navegar"
                    tileDescription="Mostrar la ubicacion en la barra de busqueda al moverde y dar click sobre el mapa."
                    icon={<GrMapLocation />}
                />
                <ListTile
                    tileName="Notificaciones"
                    tileDescription="Que te avisamos y por donde"
                    onClick={() => openPanel('Notificaciones')}
                    icon={<IoMdNotificationsOutline />}
                    disabled={true}
                />
                <ListTile
                    tileName="Ayudas y guias"
                    tileDescription="Respuestas rapidas para el equipo"
                    onClick={() => requestLeave(() => openPanel('Ayudas y guias'))}
                    icon={<FiHelpCircle />}
                />
            </OptionList>
            <AreaFilterPreferences
                preferences={draft.area}
                onPreferencesChange={(area) => setDraft((prev) => ({ ...prev, area }))}
            />
            <RowSection debug={false} arrangementSubsections='toLeft' className="gap-3 mt-5">
                <Button
                    title="Descartar"
                    format='row'
                    variant="subtle"
                    disabled={!dirty}
                    tooltip={!dirty ? "No hay cambios pendientes" : undefined}
                    onClick={resetDraft}
                />
                <Button
                    title="Guardar Cambios"
                    format='row'
                    icon={<LuSun />}
                    variant="primary"
                    disabled={!dirty}
                    tooltip={!dirty ? "No hay cambios pendientes" : undefined}
                    onClick={handleSave}
                />
            </RowSection>
        </ColSection>
    );
}

export default function UserProfileView() {
    const { user, isAuthenticated } = useAuth();

    if (!isAuthenticated || !user) {
        return (
            <div className="flex flex-row items-center justify-center gap-3 text-danger">
                <FaUserLock className="text-3xl" />
                <span className="text-xl">Acceso Denegado</span>
            </div>
        );
    }

    return <ProfilePanel key={user.id} user={user} />;
}
