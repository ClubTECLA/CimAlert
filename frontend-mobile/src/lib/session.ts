import { useSyncExternalStore } from 'react';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';
import { env } from './env';

/** `null`: todavía no elige (Bienvenida). `invitado`: solo ve el mapa y los reportes. */
export type ModoSesion = 'invitado' | 'usuario';

// Con mocks (sin backend), terminar el registro o el inicio de sesión deja esta sesión de prueba
// para poder recorrer la app como usuario. Con backend real se guarda el token de /auth/login o /auth/register.
const TOKEN_SIMULADO = 'sesion-simulada';

interface SessionState {
  token: string | null;
  modo: ModoSesion | null;
  setToken: (token: string | null) => void;
  entrarComoInvitado: () => void;
  /** Sin token (mientras no haya auth real) se usa la sesión simulada. */
  iniciarSesion: (token?: string) => void;
  /** Cierra la sesión o sale del modo invitado: vuelve a la Bienvenida. */
  clear: () => void;
  /** El backend rechazó `token` (401: venció o se cambió la contraseña). Cierra la sesión y avisa,
   * pero solo si ese sigue siendo el token actual: varias peticiones rechazadas a la vez, o una que
   * llega después de iniciar sesión de nuevo, no deben cerrar la sesión nueva ni avisar dos veces. */
  expirarSesion: (token: string) => void;
}

type AlExpirar = () => void;
const alExpirar = new Set<AlExpirar>();

/** Avisa cuando el backend rechaza la sesión. Devuelve la función para dejar de escuchar. */
export function onSesionExpirada(cb: AlExpirar): () => void {
  alExpirar.add(cb);
  return () => {
    alExpirar.delete(cb);
  };
}

export const useSessionStore = create<SessionState>()(
  persist(
    (set, get) => ({
      token: null,
      modo: null,
      setToken: (token) => set({ token }),
      entrarComoInvitado: () => set({ token: null, modo: 'invitado' }),
      iniciarSesion: (token) => {
        const efectivo = token ?? (env.useMocks ? TOKEN_SIMULADO : null);
        if (efectivo !== null) set({ token: efectivo, modo: 'usuario' });
      },
      clear: () => set({ token: null, modo: null }),
      expirarSesion: (token) => {
        if (get().token !== token) return;
        set({ token: null, modo: null });
        alExpirar.forEach((cb) => cb());
      },
    }),
    {
      // Se recuerda entre aperturas: quien ya eligió (invitado o con sesión) entra directo al mapa.
      name: 'sesion',
      storage: createJSONStorage(() => AsyncStorage),
      partialize: ({ token, modo }) => ({ token, modo }),
    },
  ),
);

/** true cuando se puede iniciar sesión sin backend (mocks activos). */
export const puedeSimularSesion = env.useMocks;

/** `false` hasta que la sesión guardada se termina de leer del almacenamiento. */
export function useSesionCargada(): boolean {
  return useSyncExternalStore(suscribirCarga, estaCargada);
}

const suscribirCarga = (alCambiar: () => void) => useSessionStore.persist.onFinishHydration(alCambiar);
const estaCargada = () => useSessionStore.persist.hasHydrated();

/** true salvo con sesión iniciada: el invitado solo puede ver el mapa y los reportes. */
export function useEsInvitado(): boolean {
  return useSessionStore((s) => s.modo !== 'usuario');
}
