import { useState, type DragEvent } from "react";
import { LuImage, LuX } from "react-icons/lu";
import { ACCEPT_ATTRIBUTE } from "@/features/reportes/prepararFoto";
import { useSysMessage } from "@/components/ui/SysMessageContext";
import LoadingIcon from "@/components/ui/LoadingIcon";

interface PhotoDropzoneProps {
    /** Vista previa de la foto o del video ya preparado; sin ella se muestra la zona para elegir. */
    preview?: string | null
    /** La vista previa es un video. */
    video?: boolean
    /** Se está preparando el archivo elegido. */
    busy?: boolean
    disabled?: boolean
    onSelect: (file: File) => void
    onRemove: () => void
}

// "Subir fotos" del Figma, con una foto o un video: el backend admite uno (uno u otro) por usuario por reporte.
// Los límites reales (formato, peso y duración) los validan prepararFoto y prepararVideo; aquí solo se recoge el archivo.
export default function PhotoDropzone({ preview, video = false, busy = false, disabled = false, onSelect, onRemove }: PhotoDropzoneProps) {
    const [dragging, setDragging] = useState(false);
    const { showMessage } = useSysMessage();

    if (preview) {
        return (
            // shrink-0: dentro de la columna con scroll del formulario, un elemento con overflow-hidden tiene altura
            // mínima 0 y se encoge (hasta desaparecer) cuando el contenido no cabe, en vez de hacer scroll.
            <div className="relative h-40 w-full shrink-0 overflow-hidden rounded-2xl border border-line bg-subtle">
                {video
                    ? <video src={preview} controls muted playsInline preload="metadata" aria-label="Video del incidente" className="size-full bg-black object-contain" />
                    : <img src={preview} alt="Foto del incidente" className="size-full object-cover" />}
                <button
                    type="button"
                    aria-label={video ? "Quitar video" : "Quitar foto"}
                    onClick={onRemove}
                    disabled={disabled}
                    className="absolute right-2 top-2 flex size-8 items-center justify-center rounded-full bg-surface text-icon shadow-raised transition hover:brightness-95 disabled:opacity-60"
                >
                    <LuX className="size-4" />
                </button>
            </div>
        );
    }

    const inactive = disabled || busy;
    const handleDrop = (event: DragEvent<HTMLLabelElement>) => {
        event.preventDefault();
        setDragging(false);
        const { files } = event.dataTransfer;
        if (!files[0] || inactive) return;
        // Al hacer clic el selector solo deja elegir una; al arrastrar pueden llegar varias.
        if (files.length > 1) {
            showMessage("Solo se puede subir una foto o un video: se usa el primero y los demás se ignoran.", { color: "yellow", type: "float", title: "Multimedia" });
        }
        onSelect(files[0]);
    };

    return (
        <label
            onDragOver={(event) => { event.preventDefault(); if (!inactive) setDragging(true) }}
            onDragLeave={() => setDragging(false)}
            onDrop={handleDrop}
            className={`
                flex w-full flex-col items-center gap-1.5 rounded-2xl
                border border-dashed p-4.5 text-center
                ${dragging ? 'border-brand bg-brand-subtle' : 'border-line-strong bg-subtle'}
                ${inactive ? 'cursor-not-allowed opacity-60' : 'cursor-pointer hover:bg-line-soft/60'}
            `}
        >
            <input
                type="file"
                accept={ACCEPT_ATTRIBUTE}
                disabled={inactive}
                className="sr-only"
                onChange={(event) => {
                    const file = event.target.files?.[0];
                    // Se vacía para que volver a elegir el mismo archivo también dispare onChange.
                    event.target.value = "";
                    if (file) onSelect(file);
                }}
            />
            {busy
                ? <LoadingIcon label="Preparando archivo" />
                : <LuImage className="size-6 text-brand-text" />}
            <span className="text-sm font-semibold text-brand-text">
                {busy ? "Preparando archivo…" : "Arrastra una foto o un video, o haz clic para subir"}
            </span>
            <span className="text-xs text-ink-muted">Foto JPG, PNG, WebP o HEIC (máx. 20 MB) · video MP4 de hasta 6 s y 8 MB</span>
        </label>
    );
}
