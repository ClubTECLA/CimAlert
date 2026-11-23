import { beforeEach, describe, expect, it, vi } from 'vitest';

// La sesión y la navegación son de la app: aquí solo importa si se abrió la sesión y con qué token.
const { iniciarSesion } = vi.hoisted(() => ({ iniciarSesion: vi.fn() }));
vi.mock('@/lib/session', () => ({
  puedeSimularSesion: false,
  useSessionStore: { getState: () => ({ iniciarSesion }) },
}));
vi.mock('expo-router', () => ({ router: { dismissTo: vi.fn() } }));

import { descartarIntentoDeSesion, esIntentoVigente, iniciarSesionYEntrar, nuevoIntentoDeSesion } from './entrar';

// Un login cuya respuesta llega tarde no debe reemplazar la sesión de un intento posterior.
describe('iniciarSesionYEntrar con intentos', () => {
  beforeEach(() => iniciarSesion.mockClear());

  it('la respuesta del intento más reciente abre la sesión', () => {
    const intento = nuevoIntentoDeSesion();
    expect(iniciarSesionYEntrar('tok-nuevo', intento)).toBe(true);
    expect(iniciarSesion).toHaveBeenCalledWith('tok-nuevo');
  });

  it('una respuesta vieja que llega después de un intento más nuevo se ignora', () => {
    const primero = nuevoIntentoDeSesion();
    const segundo = nuevoIntentoDeSesion();

    expect(iniciarSesionYEntrar('tok-segundo', segundo)).toBe(true);
    // La respuesta del primer intento llega al final: no debe pisar la sesión del segundo.
    expect(iniciarSesionYEntrar('tok-primero', primero)).toBe(false);
    expect(iniciarSesion).toHaveBeenCalledTimes(1);
    expect(iniciarSesion).toHaveBeenCalledWith('tok-segundo');
  });

  it('un intento descartado al salir de la pantalla no abre sesión cuando responde', () => {
    // Se pidió el login y, antes de que respondiera, se fue a "¿Olvidaste la contraseña?".
    const intento = nuevoIntentoDeSesion();
    descartarIntentoDeSesion();

    expect(iniciarSesionYEntrar('tok-tarde', intento)).toBe(false);
    expect(iniciarSesion).not.toHaveBeenCalled();
  });

  it('esIntentoVigente: solo el último intento sin descartar sigue vigente (para no mostrar errores viejos)', () => {
    const viejo = nuevoIntentoDeSesion();
    const nuevo = nuevoIntentoDeSesion();
    expect(esIntentoVigente(viejo)).toBe(false);
    expect(esIntentoVigente(nuevo)).toBe(true);

    descartarIntentoDeSesion();
    expect(esIntentoVigente(nuevo)).toBe(false);
  });

  it('sin token real ni mocks no abre ninguna sesión', () => {
    expect(iniciarSesionYEntrar()).toBe(false);
    expect(iniciarSesion).not.toHaveBeenCalled();
  });
});
