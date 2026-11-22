import { beforeEach, describe, expect, it, vi } from 'vitest';

// La sesión se guarda en AsyncStorage (módulo nativo); aquí basta un almacenamiento en memoria.
vi.mock('@react-native-async-storage/async-storage', () => {
  const datos = new Map<string, string>();
  return {
    default: {
      getItem: async (k: string) => datos.get(k) ?? null,
      setItem: async (k: string, v: string) => void datos.set(k, v),
      removeItem: async (k: string) => void datos.delete(k),
    },
  };
});
vi.mock('./env', () => ({ env: { apiUrl: 'http://localhost/api/v1', useMocks: false } }));

import { onSesionExpirada, useSessionStore } from './session';

// expirarSesion es lo que llama `http` cuando el backend responde 401 con un token.
describe('expirarSesion', () => {
  beforeEach(() => useSessionStore.setState({ token: 'tok-1', modo: 'usuario' }));

  it('con el token actual cierra la sesión (vuelve a la Bienvenida) y avisa', () => {
    const aviso = vi.fn();
    const dejar = onSesionExpirada(aviso);

    useSessionStore.getState().expirarSesion('tok-1');

    expect(useSessionStore.getState()).toMatchObject({ token: null, modo: null });
    expect(aviso).toHaveBeenCalledTimes(1);
    dejar();
  });

  it('varias peticiones rechazadas a la vez solo avisan una vez', () => {
    const aviso = vi.fn();
    const dejar = onSesionExpirada(aviso);

    useSessionStore.getState().expirarSesion('tok-1');
    useSessionStore.getState().expirarSesion('tok-1');

    expect(aviso).toHaveBeenCalledTimes(1);
    dejar();
  });

  it('un rechazo de un token viejo no cierra la sesión nueva', () => {
    // Una petición lenta hecha con tok-1 vuelve con 401 cuando el usuario ya entró con tok-2.
    useSessionStore.setState({ token: 'tok-2', modo: 'usuario' });
    const aviso = vi.fn();
    const dejar = onSesionExpirada(aviso);

    useSessionStore.getState().expirarSesion('tok-1');

    expect(useSessionStore.getState()).toMatchObject({ token: 'tok-2', modo: 'usuario' });
    expect(aviso).not.toHaveBeenCalled();
    dejar();
  });

  it('quien deja de escuchar ya no recibe el aviso', () => {
    const aviso = vi.fn();
    onSesionExpirada(aviso)();

    useSessionStore.getState().expirarSesion('tok-1');

    expect(aviso).not.toHaveBeenCalled();
  });
});
