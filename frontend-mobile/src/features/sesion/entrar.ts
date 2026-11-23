import { router } from 'expo-router';
import { puedeSimularSesion, useSessionStore } from '@/lib/session';

// Número del intento de login o registro más reciente. Si una respuesta llega tarde (p. ej. se salió de
// la pantalla y se entró con otra cuenta antes de que respondiera), ya no es la del último intento y se
// ignora: no debe reemplazar la sesión que se abrió después.
let ultimoIntento = 0;

/** Llamar al empezar un login o registro; el número se le pasa a `iniciarSesionYEntrar` con la respuesta. */
export function nuevoIntentoDeSesion(): number {
  ultimoIntento += 1;
  return ultimoIntento;
}

/** Inicia sesión y regresa al mapa, ya sea que el login se abrió desde la Bienvenida o desde el
 * mapa como invitado: `dismissTo` vuelve al mapa si está en el historial, o lo abre si no.
 * Con `intento`, solo entra si sigue siendo el intento más reciente. Devuelve si entró. */
export function iniciarSesionYEntrar(token?: string, intento?: number): boolean {
  if (intento !== undefined && intento !== ultimoIntento) return false;
  // Sin token real solo se puede entrar con la sesión simulada, y esa existe únicamente con mocks.
  if (token === undefined && !puedeSimularSesion) return false;
  useSessionStore.getState().iniciarSesion(token);
  // Sin sesión el mapa está protegido (app/_layout.tsx): se navega en el siguiente tick, cuando
  // el layout ya se volvió a pintar con la ruta disponible.
  setTimeout(() => router.dismissTo('/(main)'), 0);
  return true;
}
