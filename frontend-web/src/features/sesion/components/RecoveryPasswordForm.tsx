import { useState } from 'react'
import type { SubmitEvent } from 'react'
import { FiInfo } from 'react-icons/fi'
import { LuCircleCheckBig } from 'react-icons/lu'
import { getPasswordByteLength, getPasswordRequirements, isPasswordValid, PASSWORD_MAX_BYTES } from '@/features/sesion/passwordPolicy'
import { PasswordInput } from './PasswordInput'
import {
    submitButtonStyle,
    subtitleStyle,
    titleStyle,
} from './authStyles'

interface RecoveryPasswordFormProps {
    onSubmit: (password: string) => Promise<void>
    submitting: boolean
}

const PASSWORD_MISMATCH_ERROR_ID = 'recovery-password-mismatch-error'

export function RecoveryPasswordForm({ onSubmit, submitting }: RecoveryPasswordFormProps) {
    const [password, setPassword] = useState('')
    const [confirmPassword, setConfirmPassword] = useState('')
    const requirements = getPasswordRequirements(password)
    const passwordValid = isPasswordValid(password)
    const passwordBytes = getPasswordByteLength(password)
    const passwordTooLong = passwordBytes > PASSWORD_MAX_BYTES
    const passwordsMatch = password.length > 0 && password === confirmPassword
    const passwordsDiffer = confirmPassword.length > 0 && !passwordsMatch

    const handleSubmit = (event: SubmitEvent<HTMLFormElement>) => {
        event.preventDefault()
        if (passwordValid && passwordsMatch) void onSubmit(password)
    }

    return (
        <form
            className="w-full h-fit"
            onSubmit={handleSubmit}
        >
            <h1 className={titleStyle}>Crea una nueva contraseña</h1>
            <span className={subtitleStyle}>El código ya fue validado. Elige una contraseña nueva y guárdala antes de que venza el permiso (15 minutos).</span>
            <PasswordInput
                id="recovery-new-password"
                label="Nueva contraseña"
                containerClassName="mt-4"
                autoComplete="new-password"
                autoFocus
                aria-describedby={passwordTooLong
                    ? 'recovery-password-requirements recovery-password-bytes-error'
                    : 'recovery-password-requirements'}
                aria-invalid={passwordTooLong}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                disabled={submitting}
                required
            >
                {passwordTooLong && (
                    <span id="recovery-password-bytes-error" className="text-sm text-red-600" role="alert">
                        La contraseña tiene {passwordBytes} bytes; el límite es de {PASSWORD_MAX_BYTES} bytes.
                    </span>
                )}
            </PasswordInput>
            <ul
                id="recovery-password-requirements"
                className="mt-2 flex flex-col gap-1 text-sm"
                aria-label="Requisitos de contraseña"
            >
                {requirements.map(({ label, met }) => (
                    <li key={label} className={`flex items-center gap-2 ${met ? 'text-emerald-600' : 'text-ink-muted'}`}>
                        {met
                            ? <LuCircleCheckBig className="size-4 shrink-0" aria-hidden="true" />
                            : <FiInfo className="size-4 shrink-0" aria-hidden="true" />}
                        {label}
                        <span className="sr-only"> · {met ? 'Cumplido' : 'Pendiente'}</span>
                    </li>
                ))}
            </ul>
            <PasswordInput
                id="recovery-confirm-password"
                label="Confirmar contraseña"
                containerClassName="mt-4"
                autoComplete="new-password"
                className={confirmPassword.length === 0
                    ? ''
                    : passwordsMatch
                        ? 'border-emerald-500'
                        : 'border-red-500'}
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                disabled={submitting}
                aria-describedby={passwordsDiffer ? PASSWORD_MISMATCH_ERROR_ID : undefined}
                aria-invalid={passwordsDiffer}
                required
            >
                {passwordsDiffer && (
                    <span id={PASSWORD_MISMATCH_ERROR_ID} className="text-sm text-red-600" role="alert">
                        Las contraseñas no coinciden.
                    </span>
                )}
            </PasswordInput>
            <button
                className={`${submitButtonStyle} mt-5`}
                type="submit"
                disabled={!passwordValid || !passwordsMatch || submitting}
            >
                {submitting ? 'Guardando…' : 'Actualizar contraseña'}
            </button>
        </form>
    )
}
