// Revisa en el navegador el video de un reporte antes de subirlo (multimedia/README.md). El servidor solo acepta MP4
// (no .mov ni fragmentado) de máx. 8 MiB y 6 s, y no recomprime: aquí no se recorta ni se convierte, solo se avisa
// antes de crear el reporte en vez de después.
import type { PreparedMedia } from './prepararFoto'

/** MAX_VIDEO_BYTES y MAX_VIDEO_SEGUNDOS del backend (config.go), con sus valores por defecto. */
const MAX_VIDEO_BYTES = 8 * 1024 * 1024
const MAX_VIDEO_SECONDS = 6

export const VIDEO_TYPE = 'video/mp4'
export const isVideoFile = (file: File) => file.type.startsWith('video/') || /\.(mp4|mov|m4v|webm)$/i.test(file.name)
const isMp4 = (file: File) => file.type === VIDEO_TYPE || (!file.type && /\.mp4$/i.test(file.name))

// Lee la duración y el tamaño con el propio reproductor y espera al primer fotograma decodificado (loadeddata): un
// códec que este navegador no sabe decodificar (p. ej. HEVC) puede dar metadatos y fallar después, y eso lo rechaza.
const readVideo = (url: string) =>
    new Promise<{ duration: number; width: number }>((resolve, reject) => {
        const video = document.createElement('video')
        video.muted = true
        video.playsInline = true
        video.preload = 'auto'
        video.onloadeddata = () => resolve({ duration: video.duration, width: video.videoWidth })
        video.onerror = () => reject(new Error('No se pudo abrir el video. Exporta un MP4 en H.264 y vuelve a elegirlo.'))
        video.src = url
    })

/** Comprueba que el video cumpla lo que acepta el servidor; lanza un error con el motivo si no. */
export async function prepararVideo(file: File): Promise<PreparedMedia> {
    if (!isMp4(file)) {
        throw new Error('Formato de video no soportado. Usa un MP4 (los .mov de iPhone hay que exportarlos como MP4).')
    }
    if (file.size > MAX_VIDEO_BYTES) {
        throw new Error('El video pesa más de 8 MB. Recórtalo o compártelo con menos calidad.')
    }

    const preview = URL.createObjectURL(file)
    try {
        const { duration, width } = await readVideo(preview)
        if (!width) throw new Error('El archivo no tiene imagen. Elige un video.')
        if (!Number.isFinite(duration) || duration > MAX_VIDEO_SECONDS) {
            throw new Error(`El video dura más de ${MAX_VIDEO_SECONDS} segundos. Recórtalo y vuelve a elegirlo.`)
        }
    } catch (error) {
        URL.revokeObjectURL(preview)
        throw error
    }
    // Con el tipo fijado: así se sabe que es video aunque el sistema no lo informara (solo la extensión .mp4).
    return { blob: file.slice(0, file.size, VIDEO_TYPE), preview }
}
