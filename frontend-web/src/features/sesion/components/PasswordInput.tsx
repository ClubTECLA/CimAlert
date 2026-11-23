import { useState, type InputHTMLAttributes, type ReactNode } from 'react'
import { LuEye, LuEyeOff } from 'react-icons/lu'
import { formsInputsDivsStyle, inputsLabelsStyle, passwordInputsStyle } from './authStyles'

type PasswordInputProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> & {
    id: string
    label: string
    /** Clases del contenedor, para márgenes propios de cada formulario. */
    containerClassName?: string
    /** Mensajes de error de este campo, dentro del mismo bloque que el input. */
    children?: ReactNode
}

/** Campo de contraseña con botón propio para mostrarla u ocultarla. */
export function PasswordInput({
    id,
    label,
    className,
    containerClassName,
    disabled,
    children,
    ...inputProps
}: PasswordInputProps) {
    const [visible, setVisible] = useState(false)

    return (
        <div className={`${formsInputsDivsStyle} ${containerClassName ?? ''}`}>
            <label htmlFor={id} className={inputsLabelsStyle}>{label}</label>
            <div className="relative w-full">
                <input
                    {...inputProps}
                    id={id}
                    type={visible ? 'text' : 'password'}
                    disabled={disabled}
                    className={`${passwordInputsStyle} ${className ?? ''}`}
                />
                <button
                    type="button"
                    onClick={() => setVisible((current) => !current)}
                    disabled={disabled}
                    aria-label="Mostrar contraseña"
                    aria-pressed={visible}
                    title={visible ? 'Ocultar contraseña' : 'Mostrar contraseña'}
                    className="
                        absolute inset-y-0 right-0 flex w-11 items-center justify-center
                        rounded-r-[31px] text-ink-secondary transition-colors
                        hover:text-ink focus:outline-none focus-visible:ring-2 focus-visible:ring-brand
                        disabled:cursor-not-allowed disabled:opacity-60
                    "
                >
                    {visible
                        ? <LuEyeOff className="size-4.5" aria-hidden="true" />
                        : <LuEye className="size-4.5" aria-hidden="true" />}
                </button>
            </div>
            {children}
        </div>
    )
}
