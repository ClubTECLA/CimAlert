import { LuTarget, LuUser } from "react-icons/lu";
import type { ApiComentario } from "@/types/api-types";
import { formatDateTime, timeAgo } from "@/features/reportes/reportFormat";
import { initials, isOfficialRole, rolLabel } from "@/features/sesion/users";
import { Badge } from "./Badge";

export type CommentAuthor = {
    name: string
    /** rol_name del autor; si no es ciudadano, el comentario es oficial. */
    rol?: string | null
}

interface CommentItemProps {
    comentario: ApiComentario
    /** Autor, si se conoce. Solo se muestra si es oficial; el resto sale como "Anónimo". */
    author?: CommentAuthor
}

const avatarStyle = "flex size-8 shrink-0 items-center justify-center rounded-full text-2xs font-semibold";

// Comentario del detalle de incidente (Figma, pestaña "Comentarios"). Los de roles con más peso que el
// ciudadano son oficiales: van en la tarjeta roja "Comentario fijado", con su nombre y su rol, y se listan primero.
// Los de ciudadanos (o de autor desconocido) salen como "Anónimo": su nombre no se expone en el panel.
export default function CommentItem({ comentario, author }: CommentItemProps) {
    const time = (
        <time dateTime={comentario.created_at} title={formatDateTime(comentario.created_at)}>
            {timeAgo(comentario.created_at)}
        </time>
    );

    if (author && isOfficialRole(author.rol)) {
        return (
            <article className="flex flex-col gap-1.5 rounded-2xl bg-danger-subtle px-3.5 py-3">
                <div className="flex items-center gap-2">
                    <span className={`${avatarStyle} bg-brand text-white`} aria-hidden>{initials(author.name)}</span>
                    <span className="min-w-0 flex-1 truncate text-sm font-semibold text-ink">{author.name}</span>
                    <Badge tone="danger">{rolLabel(author.rol ?? "")}</Badge>
                    <LuTarget className="size-3.5 shrink-0 text-danger" aria-label="Comentario oficial" />
                </div>
                <p className="whitespace-pre-line wrap-break-word text-sm leading-5 text-ink">{comentario.comentario}</p>
                <span className="text-xs text-danger">Fijado arriba automáticamente · {time}</span>
            </article>
        );
    }

    return (
        <article className="flex gap-3">
            <span className={`${avatarStyle} bg-subtle text-ink-muted`} aria-hidden><LuUser className="size-4" /></span>
            <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                <div className="flex items-baseline gap-2">
                    <span className="text-sm font-semibold text-ink">Anónimo</span>
                    <span className="text-xs text-ink-muted">{time}</span>
                </div>
                <p className="whitespace-pre-line wrap-break-word text-sm text-ink-secondary">{comentario.comentario}</p>
            </div>
        </article>
    );
}
