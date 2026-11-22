/** Dígitos del código de recuperación de contraseña (igual que en la web). */
export const OTP_LENGTH = 6;

/**
 * Lo que se escribió o pegó en la casilla, sin el dígito que ya tenía. `cursor` es dónde estaba el
 * cursor antes de escribir: después del dígito (lo normal, OtpInput lo fija ahí) llega "dígito
 * anterior + lo nuevo" ("04", "0482234"); antes del dígito (0) llega al revés ("40", "4822340").
 * Usar la posición evita adivinar cuando el código pegado empieza y termina con ese mismo dígito.
 */
function sinDigitoAnterior(limpio: string, anterior: string, cursor: number): string {
  if (!anterior || limpio.length <= anterior.length) return limpio;
  const alFinal = () => (limpio.endsWith(anterior) ? limpio.slice(0, -anterior.length) : null);
  const alInicio = () => (limpio.startsWith(anterior) ? limpio.slice(anterior.length) : null);
  return (cursor === 0 ? alFinal() ?? alInicio() : alInicio() ?? alFinal()) ?? limpio;
}

/**
 * Aplica lo que el teclado dejó en la casilla `index` y dice a qué casilla pasar el foco (`null`: se queda).
 *
 * - Un dígito en una casilla vacía la llena y pasa a la siguiente.
 * - Un dígito en una casilla llena la reemplaza.
 * - El código completo pegado en cualquier casilla (vacía o llena) se reparte en todas.
 */
export function aplicarTexto(
  digitos: string[],
  index: number,
  texto: string,
  cursor?: number,
): { digitos: string[]; foco: number | null } {
  const limpio = texto.replace(/[^0-9]/g, '');
  const anterior = digitos[index] ?? '';
  // Exactamente un código completo: reemplazó el contenido de la casilla (p. ej. el autocompletado del
  // código por SMS), así que no trae el dígito anterior y no hay nada que quitar.
  const nuevo = limpio.length === OTP_LENGTH ? limpio : sinDigitoAnterior(limpio, anterior, cursor ?? anterior.length);

  if (nuevo.length >= OTP_LENGTH) {
    return { digitos: nuevo.slice(0, OTP_LENGTH).split(''), foco: OTP_LENGTH - 1 };
  }

  const siguiente = digitos.slice();
  siguiente[index] = nuevo.slice(-1);
  const foco = siguiente[index] && index < OTP_LENGTH - 1 ? index + 1 : null;
  return { digitos: siguiente, foco };
}
