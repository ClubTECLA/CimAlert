import { env } from './env';
import { useSessionStore } from './session';

// Sin respuesta en este tiempo se da la petición por perdida: sin esto, una red que "conecta"
// pero no entrega (señal débil, wifi con portal cautivo) deja la pantalla cargando para siempre.
const TIEMPO_LIMITE_MS = 15_000;

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

/** La petición no llegó al servidor (sin internet, servidor caído o tiempo agotado). */
export class ErrorRed extends Error {
  constructor(message = 'No hay conexión con el servidor.') {
    super(message);
    this.name = 'ErrorRed';
  }
}

/** `fetch` que convierte los fallos de red y el tiempo agotado en `ErrorRed`. */
export async function fetchConRed(
  url: string,
  init: RequestInit = {},
  tiempoLimiteMs: number = TIEMPO_LIMITE_MS,
): Promise<Response> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), tiempoLimiteMs);
  // La señal del llamador también cancela, sin quitarle el tiempo límite a la petición.
  const delLlamador = init.signal;
  const cancelar = () => controller.abort();
  if (delLlamador?.aborted) controller.abort();
  else delLlamador?.addEventListener('abort', cancelar);
  try {
    return await fetch(url, { ...init, signal: controller.signal });
  } catch (err) {
    // Cancelación pedida por el llamador: no es un fallo de red.
    if (delLlamador?.aborted) throw err;    // `fetch` solo rechaza cuando no hubo respuesta: los errores HTTP llegan como `response.ok === false`.
    throw new ErrorRed();
  } finally {
    clearTimeout(timer);
    delLlamador?.removeEventListener('abort', cancelar);
  }
}

export async function http<T>(
  path: string,
  init: RequestInit = {},
  { tiempoLimiteMs }: { tiempoLimiteMs?: number } = {},
): Promise<T> {
  const token = useSessionStore.getState().token;
  // Con un formulario (subir una foto) `fetch` fija el Content-Type con su boundary; forzar JSON lo rompería.
  const esFormulario = init.body instanceof FormData;

  const headers: Record<string, string> = {
    ...(!esFormulario && { 'Content-Type': 'application/json' }),
    ...(token && { Authorization: `Bearer ${token}` }),
  };
  // Los headers de quien llama van al final y reemplazan a los de arriba aunque cambien las mayúsculas
  // ("authorization" sustituye a "Authorization"): HTTP no las distingue y no deben quedar las dos.
  for (const [nombre, valor] of Object.entries((init.headers as Record<string, string> | undefined) ?? {})) {
    for (const k of Object.keys(headers)) if (k.toLowerCase() === nombre.toLowerCase()) delete headers[k];
    headers[nombre] = valor;
  }
  // El token que de verdad se mandó: quien llama puede poner su propio Authorization.
  const autorizacion = Object.entries(headers).find(([k]) => k.toLowerCase() === 'authorization')?.[1];
  const tokenEnviado = autorizacion?.startsWith('Bearer ') ? autorizacion.slice('Bearer '.length) : null;

  const response = await fetchConRed(`${env.apiUrl}${path}`, { ...init, headers }, tiempoLimiteMs);

  if (!response.ok) {
    // Con sesión, un 401 significa que el token ya no sirve (venció a las 24 h o se cambió la
    // contraseña): se cierra la sesión en el teléfono en vez de seguir fallando en cada pantalla.
    // Solo si el rechazado fue el token de la sesión: expirarSesion además ignora uno que ya no es el actual.
    if (response.status === 401 && tokenEnviado) useSessionStore.getState().expirarSesion(tokenEnviado);
    const body = (await response.json().catch(() => null)) as { message?: string } | null;
    throw new ApiError(response.status, body?.message ?? `HTTP ${response.status}`);
  }

  // Sin cuerpo (204, o 202 de POST /auth/recovery) no hay JSON que leer.
  const cuerpo = await response.text();
  return (cuerpo ? JSON.parse(cuerpo) : undefined) as T;
}
