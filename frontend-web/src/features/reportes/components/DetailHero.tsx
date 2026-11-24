import type { ReactNode } from "react";
import { LuPlay } from "react-icons/lu";
import PanelNavButton from "@/components/layout/PanelNavButton";

interface DetailHeroProps {
    /** Color de fondo mientras no hay foto (p. ej. el de la categoría). */
    color: string
    /** Icono grande y tenue en el centro, cuando no hay foto. */
    icon: ReactNode
    /** URL de la foto del reporte; sin ella se usan el color y el icono. */
    photo?: string
    /** La URL de photo es un video: se muestra su primer fotograma y se reproduce al abrirlo. */
    video?: boolean
    /** Con él, la foto abre el visor de fotos y videos. */
    onOpen?: () => void
    /** Píldoras de la esquina inferior (origen, número de fotos...). */
    children?: ReactNode
}

const heroButtonStyle = "absolute top-4 bg-surface shadow-raised hover:brightness-95";

// Cabecera de los paneles de detalle del Figma ("Panel · Detalle de incidente", también en Refugio):
// foto de 200 px con regresar y cerrar encima.
export default function DetailHero({ color, icon, photo, video = false, onOpen, children }: DetailHeroProps) {
    const media = photo && (video
        // #t=0.1: sin reproducir, algunos navegadores no pintan el primer fotograma.
        ? <>
            <video src={`${photo}#t=0.1`} muted preload="metadata" aria-hidden className="absolute inset-0 size-full bg-black object-cover" />
            <span className="absolute inset-0 m-auto flex size-12 items-center justify-center rounded-full bg-black/55 text-white" aria-hidden>
                <LuPlay className="size-6" />
            </span>
        </>
        : <img src={photo} alt={onOpen ? "" : "Foto del incidente"} className="absolute inset-0 size-full object-cover" />);

    return (
        <div className="relative h-50 w-full shrink-0 overflow-hidden rounded-[20px]" style={{ backgroundColor: color }}>
            {!media
                ? <span className="absolute inset-0 flex items-center justify-center text-white/25 [&>svg]:size-28" aria-hidden>{icon}</span>
                : onOpen
                    ? <button type="button" onClick={onOpen} aria-label={video ? "Ver el video del incidente" : "Ver las fotos del incidente"} className="absolute inset-0 cursor-zoom-in">
                        {media}
                    </button>
                    : media}
            {/* pointer-events-none: el degradado y las píldoras no deben tapar el clic que abre el visor. */}
            <span className="pointer-events-none absolute inset-0 bg-linear-to-b from-black/35 via-transparent via-40% to-black/25" aria-hidden />
            <PanelNavButton kind="back" className={`left-4 ${heroButtonStyle}`} />
            <PanelNavButton kind="close" className={`right-4 ${heroButtonStyle}`} />
            <div className="pointer-events-none absolute inset-x-4 bottom-4 flex flex-wrap items-center justify-between gap-2">
                {children}
            </div>
        </div>
    );
}
