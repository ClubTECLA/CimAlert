import { ApiError, ErrorRed, http } from '@/lib/http';

/** Respuesta de POST /auth/login y /auth/register (shared/openapi/schemas/auth/auth_response.yaml). */
export interface AuthResponseApi {
  access_token: string;
  token_type: string;
  expires_in: number;
  user: { id: string; email: string; username: string };
}

// Todas son rutas públicas (security: [] en el contrato): van sin el token de la sesión (`auth: false`).
// Así, si alguien con sesión abierta se equivoca de contraseña, el 401 no le cierra la sesión.
const publica = { auth: false } as const;

export const sesionApi = {
  login: (email: string, password: string) =>
    http<AuthResponseApi>('/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }, publica),
  register: (email: string, username: string, password: string) =>
    http<AuthResponseApi>(
      '/auth/register',
      { method: 'POST', body: JSON.stringify({ email, username, password }) },
      publica,
    ),
  // El backend no guarda el token: solo responde 204.
  logout: () => http<void>('/auth/logout', { method: 'POST' }, publica),
  /** Manda un PIN de 6 dígitos al correo. Responde 202 exista o no la cuenta, para no revelar cuáles existen. */
  solicitarRecuperacion: (email: string) =>
    http<void>('/auth/recovery', { method: 'POST', body: JSON.stringify({ email }) }, publica),
  /** Valida el PIN y cambia la contraseña en una sola llamada (204). No devuelve token: hay que iniciar sesión. */
  verificarRecuperacion: (email: string, pin: string, newPassword: string) =>
    http<void>(
      '/auth/recovery/verify',
      { method: 'POST', body: JSON.stringify({ email, pin, new_password: newPassword }) },
      publica,
    ),
};

/** Lo que vence y cuánto se puede intentar (backend/internal/usuario/recovery.go). */
export const RECUPERACION = { minutosVigencia: 15, intentosPin: 5, codigosPorHora: 3 } as const;

/** El 400 de POST /auth/recovery/verify por PIN incorrecto, vencido o sin intentos (el backend no distingue). */
export function esErrorDeCodigo(error: unknown): boolean {
  // El otro 400 posible es el de la contraseña (8 a 72 bytes); ese menciona "password".
  return error instanceof ApiError && error.status === 400 && !/password/i.test(error.message);
}

/** Mensaje para el usuario según el error del login, el registro o la recuperación de contraseña. */
export function mensajeErrorAuth(error: unknown, accion: 'login' | 'registro' | 'recuperacion'): string {
  if (error instanceof ErrorRed) return 'No hay conexión con el servidor. Inténtalo de nuevo.';
  if (error instanceof ApiError) {
    if (accion === 'login' && error.status === 401) return 'Correo o contraseña incorrectos.';
    if (accion === 'registro' && error.status === 409) return 'Ese correo ya está registrado.';
    if (error.status === 429) return 'Hiciste demasiados intentos. Espera unos minutos y vuelve a intentarlo.';
    if (accion === 'recuperacion' && esErrorDeCodigo(error)) {
      return 'El código es incorrecto o ya venció. Revísalo o pide uno nuevo.';
    }
    if (error.status === 400) return error.message;
  }
  return 'Algo salió mal. Inténtalo de nuevo.';
}
