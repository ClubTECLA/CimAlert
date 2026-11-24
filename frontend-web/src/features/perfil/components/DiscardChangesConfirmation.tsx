import Button from "@/components/ui/Button";
import SubSection from "@/components/layout/SubSection";
import useModals from "@/panels/useModals";

interface DiscardChangesConfirmationProps {
    /** Se llama después de aceptar: restaura el borrador y ejecuta la navegación pendiente. */
    onDiscard: () => void;
}

/** Confirmación al salir de "Mi perfil" con cambios sin guardar. Se abre con `fullFocus: true`. */
export default function DiscardChangesConfirmation({ onDiscard }: DiscardChangesConfirmationProps) {
    const { closeModal } = useModals();

    const discard = () => {
        onDiscard();
        closeModal();
    };

    return (
        <SubSection
            as="div"
            role="alertdialog"
            aria-modal="true"
            aria-labelledby="discard-changes-title"
            aria-describedby="discard-changes-description"
            className="w-[min(26rem,calc(100vw-2rem))] gap-4 p-5 flex-col"
        >
            <div className="flex flex-col gap-2">
                <h2 id="discard-changes-title" className="text-lg font-bold text-ink">Cambios sin guardar</h2>
                <p id="discard-changes-description" className="text-sm text-ink-muted">
                    Se descartarán todos los cambios que no hayas guardado en “Mi perfil”. Esta acción no se puede deshacer.
                </p>
            </div>
            <div className="flex justify-end gap-2">
                <Button title="Seguir editando" onClick={closeModal} />
                <Button title="Descartar cambios" variant="danger" onClick={discard} />
            </div>
        </SubSection>
    );
}
