/** Segundos que hay que esperar para solicitar otro código de recuperación. */
export const RESEND_SECONDS = 45

/** Segundos que quedan para reenviar, derivados de una hora límite para sobrevivir a pausas de la pestaña. */
export function resendSecondsRemaining(resendAt: number, now: number): number {
    return Math.max(Math.ceil((resendAt - now) / 1000), 0)
}
