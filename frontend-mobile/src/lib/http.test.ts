import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// La sesión persiste en AsyncStorage (módulo nativo): aquí solo importa el token.
const { expirarSesion } = vi.hoisted(() => ({ expirarSesion: vi.fn() }));
vi.mock('./session', () => ({ useSessionStore: { getState: () => ({ token: 'tok-123', expirarSesion }) } }));
vi.mock('./env', () => ({ env: { apiUrl: 'http://localhost/api/v1', useMocks: false } }));

import { ErrorRed, http } from './http';

const respuestaJson = (cuerpo: unknown, status = 200) =>
  new Response(JSON.stringify(cuerpo), { status, headers: { 'Content-Type': 'application/json' } });

const cabecerasDe = (fetchMock: ReturnType<typeof vi.fn>): Record<string, string> =>
  (fetchMock.mock.calls[0][1] as RequestInit).headers as Record<string, string>;

describe('http', () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn(async () => respuestaJson({ ok: true }));
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it('manda JSON por defecto', async () => {
    await http('/reportes', { method: 'POST', body: JSON.stringify({ a: 1 }) });

    expect(cabecerasDe(fetchMock)['Content-Type']).toBe('application/json');
  });

  it('siempre manda el token de la sesión', async () => {
    await http('/reportes');

    expect(cabecerasDe(fetchMock).Authorization).toBe('Bearer tok-123');
  });

  it('con un formulario NO fija el Content-Type: fetch pone el boundary del multipart', async () => {
    await http('/reportes/r-1/multimedia', { method: 'POST', body: new FormData() });

    expect(Object.keys(cabecerasDe(fetchMock)).map((k) => k.toLowerCase())).not.toContain('content-type');
  });

  it('con un formulario conserva el token', async () => {
    await http('/reportes/r-1/multimedia', { method: 'POST', body: new FormData() });

    expect(cabecerasDe(fetchMock).Authorization).toBe('Bearer tok-123');
  });

  it('devuelve el JSON de la respuesta', async () => {
    await expect(http('/reportes')).resolves.toEqual({ ok: true });
  });

  it.each([202, 204])('una respuesta %i sin cuerpo devuelve undefined', async (status) => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status }));

    await expect(http('/auth/recovery', { method: 'POST' })).resolves.toBeUndefined();
  });

  describe('sesión rechazada', () => {
    beforeEach(() => expirarSesion.mockClear());

    it('un 401 con sesión cierra la sesión con el token que se mandó y sigue lanzando el error', async () => {
      fetchMock.mockResolvedValueOnce(respuestaJson({ message: 'no autenticado' }, 401));

      await expect(http('/auth/me')).rejects.toMatchObject({ status: 401 });
      expect(expirarSesion).toHaveBeenCalledWith('tok-123');
    });

    it('si quien llama manda su propio token, el 401 se reporta con ese token y no con el de la sesión', async () => {
      fetchMock.mockResolvedValueOnce(respuestaJson({ message: 'no autenticado' }, 401));

      await expect(http('/auth/me', { headers: { Authorization: 'Bearer otro' } })).rejects.toMatchObject({ status: 401 });
      // expirarSesion('otro') no cierra la sesión guardada (tok-123): solo actúa si es el token actual.
      expect(expirarSesion).toHaveBeenCalledWith('otro');
    });

    it('un authorization en minúsculas reemplaza al de la sesión: se manda y se reporta solo ese', async () => {
      fetchMock.mockResolvedValueOnce(respuestaJson({ message: 'no autenticado' }, 401));

      await expect(http('/auth/me', { headers: { authorization: 'Bearer otro' } })).rejects.toMatchObject({ status: 401 });
      const enviados = cabecerasDe(fetchMock);
      expect(Object.keys(enviados).filter((k) => k.toLowerCase() === 'authorization')).toEqual(['authorization']);
      expect(expirarSesion).toHaveBeenCalledWith('otro');
      expect(expirarSesion).not.toHaveBeenCalledWith('tok-123');
    });

    it('una ruta pública (auth: false) no manda el token y su 401 no cierra la sesión', async () => {
      // Alguien con sesión abierta se equivoca de contraseña en el login: 401 por credenciales.
      fetchMock.mockResolvedValueOnce(respuestaJson({ message: 'credenciales invalidas' }, 401));

      await expect(http('/auth/login', { method: 'POST' }, { auth: false })).rejects.toMatchObject({ status: 401 });
      expect(cabecerasDe(fetchMock).Authorization).toBeUndefined();
      expect(expirarSesion).not.toHaveBeenCalled();
    });

    it('en una ruta pública, ni un Authorization puesto a mano hace que el 401 cierre la sesión', async () => {
      fetchMock.mockResolvedValueOnce(respuestaJson({ message: 'credenciales invalidas' }, 401));

      await expect(
        http('/auth/login', { method: 'POST', headers: { Authorization: 'Bearer tok-123' } }, { auth: false }),
      ).rejects.toMatchObject({ status: 401 });
      expect(expirarSesion).not.toHaveBeenCalled();
    });

    it('otros errores (403, 500) no tocan la sesión', async () => {
      fetchMock.mockResolvedValueOnce(respuestaJson({ message: 'prohibido' }, 403));
      await expect(http('/reportes')).rejects.toMatchObject({ status: 403 });
      expect(expirarSesion).not.toHaveBeenCalled();
    });
  });

  describe('tiempo límite', () => {
    // Un fetch que nunca responde y obedece la cancelación, como el real.
    const fetchColgado = () =>
      vi.fn(
        (_url: string, init: RequestInit) =>
          new Promise<Response>((_resolve, reject) => {
            init.signal?.addEventListener('abort', () => reject(new DOMException('abortado', 'AbortError')));
          }),
      );

    it('da por perdida una petición normal a los 15 s', async () => {
      vi.useFakeTimers();
      vi.stubGlobal('fetch', fetchColgado());

      const espera = expect(http('/reportes')).rejects.toBeInstanceOf(ErrorRed);
      await vi.advanceTimersByTimeAsync(15_000);

      await espera;
    });

    it('permite un tiempo límite mayor, para subir una foto con señal débil', async () => {
      vi.useFakeTimers();
      vi.stubGlobal('fetch', fetchColgado());
      let resuelta = false;

      const peticion = http('/reportes/r-1/multimedia', { method: 'POST', body: new FormData() }, { tiempoLimiteMs: 60_000 });
      peticion.catch(() => undefined).finally(() => (resuelta = true));
      await vi.advanceTimersByTimeAsync(30_000);
      expect(resuelta).toBe(false);

      await vi.advanceTimersByTimeAsync(30_000);
      await expect(peticion).rejects.toBeInstanceOf(ErrorRed);
    });
  });
});
