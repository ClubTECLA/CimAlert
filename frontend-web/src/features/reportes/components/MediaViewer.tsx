import { useEffect, useRef, type KeyboardEvent, type MouseEvent } from "react";
import { LuChevronLeft, LuChevronRight, LuPlay, LuTrash2, LuX } from "react-icons/lu";
import type { ApiMultimedia } from "@/types/api-types";
import { esVideoPath, fotoUrl, shortDate } from "@/features/reportes/reportFormat";

interface MediaViewerProps {
    items: ApiMultimedia[]
    /** Archivo abierto; lo controla quien abre el visor. */
    index: number
    onIndexChange: (index: number) => void
    onClose: () => void
    /** Lugar del incidente: el backend no guarda dónde se tomó cada foto. */
    place?: string
}

const roundButton = "flex items-center justify-center rounded-full bg-white/12 text-white transition hover:bg-white/20";

// "VisorFotos" del Figma: el archivo en grande, anterior y siguiente, sus datos y las miniaturas. Es un <dialog> modal:
// el navegador ya atrapa el foco, lo devuelve al cerrar, cierra con Esc y lo pone por encima de todo.
export default function MediaViewer({ items, index, onIndexChange, onClose, place }: MediaViewerProps) {
    const dialogRef = useRef<HTMLDialogElement>(null);
    const current = items[index];
    const video = esVideoPath(current.path);
    const several = items.length > 1;
    const go = (step: number) => onIndexChange((index + step + items.length) % items.length);

    // close() al desmontar devuelve el foco a la cabecera que abrió el visor.
    useEffect(() => {
        const dialog = dialogRef.current;
        dialog?.showModal();
        return () => dialog?.close();
    }, []);

    const handleKeyDown = (event: KeyboardEvent) => {
        // En los controles del video las flechas adelantan o atrasan; ahí no cambian de archivo.
        if (!several || event.target instanceof HTMLVideoElement) return;
        if (event.key === "ArrowLeft") go(-1);
        if (event.key === "ArrowRight") go(1);
    };

    // Clic en el fondo (fuera del archivo, los botones y las miniaturas): cierra, como en el Figma.
    const handleClick = (event: MouseEvent) => {
        if (event.target instanceof HTMLElement && "backdrop" in event.target.dataset) onClose();
    };

    return (
        <dialog
            ref={dialogRef}
            aria-label="Fotos y videos del incidente"
            // El evento close llega después: si el diálogo ya se volvió a abrir (React monta los efectos dos veces en
            // desarrollo) no es un cierre de verdad.
            onClose={() => { if (!dialogRef.current?.open) onClose() }}
            onKeyDown={handleKeyDown}
            onClick={handleClick}
            // Fondo del Figma (rgba 5 8 15 / 0.92): sin token propio, es el único fondo de este tipo.
            className="m-0 size-full max-h-none max-w-none bg-[rgb(5_8_15/0.92)] p-0 font-ui text-white backdrop:bg-transparent"
        >
            {/* Primero en el orden: es lo que enfoca el navegador al abrir el visor. */}
            <button type="button" aria-label="Cerrar" onClick={onClose} className={`${roundButton} absolute right-4 top-4 z-10 size-12 sm:right-8 sm:top-8`}>
                <LuX className="size-5.5" />
            </button>
            <div data-backdrop className="flex size-full flex-col items-center justify-center gap-5 px-4 py-20 sm:px-6">
                <div data-backdrop className="relative flex min-h-0 w-full items-center justify-center gap-11">
                    {several &&
                        <button type="button" aria-label="Anterior" onClick={() => go(-1)}
                            className={`${roundButton} z-10 size-14 shrink-0 max-sm:absolute max-sm:left-2`}>
                            <LuChevronLeft className="size-6" />
                        </button>
                    }
                    {/* key: al cambiar de archivo el reproductor empieza de cero en vez de seguir con el anterior. */}
                    {video
                        ? <video key={current.path} src={fotoUrl(current.path)} controls autoPlay playsInline
                            aria-label={`Video ${index + 1} de ${items.length}`}
                            className="max-h-[min(700px,calc(100dvh-20rem))] w-full max-w-280 rounded-[20px] bg-black object-contain" />
                        : <img key={current.path} src={fotoUrl(current.path)} alt={`Foto ${index + 1} de ${items.length}`}
                            className="max-h-[min(700px,calc(100dvh-20rem))] w-auto max-w-[min(70rem,100%)] rounded-[20px] object-contain" />}
                    {several &&
                        <button type="button" aria-label="Siguiente" onClick={() => go(1)}
                            className={`${roundButton} z-10 size-14 shrink-0 max-sm:absolute max-sm:right-2`}>
                            <LuChevronRight className="size-6" />
                        </button>
                    }
                </div>

                <div data-backdrop className="flex w-full max-w-280 flex-wrap items-center justify-between gap-x-4 gap-y-2">
                    <div className="flex min-w-0 items-center gap-4" aria-live="polite">
                        <span className="shrink-0 text-base font-bold">{video ? "Video" : "Foto"} {index + 1} de {items.length}</span>
                        <span className="truncate text-sm text-line-strong">
                            {[shortDate(current.created_at), place].filter(Boolean).join(" · ")}
                        </span>
                    </div>
                    {/* Sin endpoint para retirar u ocultar un archivo: se muestra como en el Figma, pero desactivado. */}
                    <button type="button" disabled title="Próximamente"
                        className="flex shrink-0 cursor-not-allowed items-center gap-2 rounded-full bg-white/12 px-4 py-2.5 text-sm font-semibold opacity-60">
                        <LuTrash2 className="size-4" aria-hidden />
                        Retirar {video ? "video" : "foto"}
                    </button>
                </div>

                {several &&
                    <div className="flex max-w-full gap-2.5 overflow-x-auto p-1">
                        {items.map((item, i) =>
                            <button
                                key={item.id}
                                type="button"
                                aria-label={`${esVideoPath(item.path) ? "Video" : "Foto"} ${i + 1} de ${items.length}`}
                                aria-current={i === index}
                                onClick={() => onIndexChange(i)}
                                className={`relative h-18 w-24 shrink-0 overflow-hidden rounded-[10px] transition
                                    ${i === index ? "border-3 border-white" : "opacity-60 hover:opacity-90"}`}
                            >
                                {esVideoPath(item.path)
                                    // #t=0.1: sin reproducir, algunos navegadores no pintan el primer fotograma.
                                    ? <>
                                        <video src={`${fotoUrl(item.path)}#t=0.1`} muted preload="metadata" className="size-full object-cover" />
                                        <LuPlay className="absolute inset-0 m-auto size-6 drop-shadow" aria-hidden />
                                    </>
                                    : <img src={fotoUrl(item.path)} alt="" className="size-full object-cover" />}
                            </button>
                        )}
                    </div>
                }
            </div>

        </dialog>
    );
}
