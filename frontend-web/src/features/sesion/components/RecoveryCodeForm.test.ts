import { describe, expect, it } from 'vitest'
import { resendSecondsRemaining } from '@/features/sesion/resendCooldown'

describe('resendSecondsRemaining', () => {
    const start = 1_000_000

    it('mantiene el plazo de 45 segundos y baja segundo a segundo', () => {
        const resendAt = start + 45_000
        expect(resendSecondsRemaining(resendAt, start)).toBe(45)
        expect(resendSecondsRemaining(resendAt, start + 1_000)).toBe(44)
        expect(resendSecondsRemaining(resendAt, start + 44_500)).toBe(1)
    })

    it('sigue descontando aunque un tick no cambie el número redondeado', () => {
        const resendAt = start + 45_000
        expect(resendSecondsRemaining(resendAt, start + 200)).toBe(45)
        expect(resendSecondsRemaining(resendAt, start + 1_200)).toBe(44)
    })

    it('libera el reenvío al vencer, aunque la pestaña estuviera suspendida', () => {
        const resendAt = start + 45_000
        expect(resendSecondsRemaining(resendAt, start + 105_000)).toBe(0)
        expect(resendSecondsRemaining(resendAt, start + 45_000)).toBe(0)
    })
})
