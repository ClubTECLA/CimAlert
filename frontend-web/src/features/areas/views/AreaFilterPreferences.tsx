import { useState, type ChangeEvent } from "react";
import { SegmentedTabs, TabItem } from "@/components/ui/SegmentedTabs";
import type { AreaFilterMode } from "@/types/localStorage-types";
import {
    AREA_THRESHOLD_MAX,
    AREA_THRESHOLD_MIN,
    AREA_THRESHOLD_STEP,
    DEFAULT_AREA_PREFERENCES,
    normalizeAreaPreferences,
    type AreaPreferences,
} from "@/features/areas/areaPreferences";

const MODES: { key: AreaFilterMode; label: string }[] = [
    { key: "auto", label: "Automático" },
    { key: "frontend", label: "Navegador" },
    { key: "backend", label: "Servidor" },
];

interface AreaFilterPreferencesProps {
    /** Borrador del panel "Mi perfil"; los cambios solo se guardan al pulsar "Guardar Cambios". */
    preferences: AreaPreferences;
    onPreferencesChange: (next: AreaPreferences) => void;
}

// El borrador vive en UserProfileView: aquí solo se edita y se notifica. El texto del umbral se mantiene aparte
// para poder escribir valores incompletos; al perder el foco se normaliza al rango permitido.
export default function AreaFilterPreferences({ preferences, onPreferencesChange }: AreaFilterPreferencesProps) {
    const current = preferences;
    const [thresholdText, setThresholdText] = useState(() => String(current.threshold));
    const [syncedThreshold, setSyncedThreshold] = useState(current.threshold);

    // Ajuste durante el render (no en un efecto) para reflejar guardados, descartes y cambios de otra pestaña.
    if (current.threshold !== syncedThreshold) {
        setSyncedThreshold(current.threshold);
        setThresholdText(String(current.threshold));
    }

    const update = (next: Partial<AreaPreferences>) => {
        onPreferencesChange(normalizeAreaPreferences({ ...current, ...next }));
    };

    const handleThresholdChange = (event: ChangeEvent<HTMLInputElement>) => {
        const text = event.target.value;
        setThresholdText(text);
        const parsed = Number(text);
        // Valor en bruto mientras se escribe: normalizar aquí haría saltar "2" a "10" antes de escribir "25".
        if (text.trim() !== "" && Number.isFinite(parsed)) {
            onPreferencesChange({ ...current, threshold: parsed });
        }
    };

    const commitThreshold = () => {
        const parsed = Number(thresholdText);
        const value = thresholdText.trim() !== "" && Number.isFinite(parsed) ? parsed : current.threshold;
        const normalized = normalizeAreaPreferences({ ...current, threshold: value });
        setThresholdText(String(normalized.threshold));
        onPreferencesChange(normalized);
    };

    return (
        <div className="mt-2 flex flex-col gap-1.5">
            <span className="text-body font-semibold text-ink-muted">Filtrado por área</span>
            <div className="mt-2 flex flex-col gap-4 rounded-2xl p-3 outline-1 outline-line">
                <div className="flex flex-col gap-1.5">
                    <span className="text-caption font-semibold text-ink">Modo de procesamiento</span>
                    <SegmentedTabs
                        format="row"
                        activeTab={MODES.findIndex(({ key }) => key === current.mode)}
                        onChange={(index) => update({ mode: MODES[index].key })}
                    >
                        {MODES.map(({ key, label }) => <TabItem key={key} tabTitle={label} />)}
                    </SegmentedTabs>
                    <p className="text-caption text-ink-muted">
                        Automático elige según el total de reportes; las otras opciones fuerzan el modo.
                    </p>
                </div>

                <div className="flex flex-col gap-1.5">
                    <label htmlFor="area-threshold" className="text-caption font-semibold text-ink">
                        Considerar “muchos” desde
                    </label>
                    <div className="flex items-center gap-2 rounded-full border border-line-strong bg-white px-3 py-1.5">
                        <input
                            id="area-threshold"
                            type="number"
                            inputMode="numeric"
                            min={AREA_THRESHOLD_MIN}
                            max={AREA_THRESHOLD_MAX}
                            step={AREA_THRESHOLD_STEP}
                            value={thresholdText}
                            onChange={handleThresholdChange}
                            onBlur={commitThreshold}
                            aria-describedby="area-threshold-help"
                            className="w-full bg-transparent text-ink outline-none"
                        />
                        <span className="shrink-0 text-caption text-ink-muted">reportes</span>
                    </div>
                    <p id="area-threshold-help" className="text-caption text-ink-muted">
                        Entre {AREA_THRESHOLD_MIN} y {AREA_THRESHOLD_MAX}. Predeterminado: {DEFAULT_AREA_PREFERENCES.threshold}. Solo se usa en modo automático.
                    </p>
                </div>
            </div>
        </div>
    );
}
