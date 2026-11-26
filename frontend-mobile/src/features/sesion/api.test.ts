import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/lib/session', () => ({ useSessionStore: { getState: () => ({ token: null }) } }));
vi.mock('@/lib/env', () => ({ env: { apiUrl: 'http://localhost', useMocks: false } }));

import { ApiError, ErrorRed } from '@/lib/http';
import { esErrorDeCodigo, mensajeErrorAuth, sesionApi } from './api';

describe('recuperación de contraseña', () => {
  afterEach(() => vi.unstubAllGlobals());

  const responder = (status: number, body?: unknown) => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
      })
    );
    vi.stubGlobal('fetch', fetchMock);
    return fetchMock;
  };

  it('verificar manda solo el correo y el PIN y devuelve el token de reset', async () => {
    const fetchMock = responder(200, { reset_token: 'tok' });
    await expect(sesionApi.verificarRecuperacion('a@b.mx', '123456')).resolves.toEqual({ reset_token: 'tok' });
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toMatch(/\/auth\/recovery\/verify$/);
    expect(JSON.parse(init.body)).toEqual({ email: 'a@b.mx', pin: '123456' });
  });

  it('restablecer manda el token y la nueva contraseña a /recovery/reset', async () => {
    const fetchMock = responder(204);
    await sesionApi.restablecerContrasena('tok', 'nueva-clave');
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toMatch(/\/auth\/recovery\/reset$/);
    expect(JSON.parse(init.body)).toEqual({ reset_token: 'tok', new_password: 'nueva-clave' });
  });
});

describe('esErrorDeCodigo', () => {
  it('es el 400 del PIN', () => {
    expect(esErrorDeCodigo(new ApiError(400, 'código inválido o expirado'))).toBe(true);
  });

  it('no es el 400 de la contraseña ni otros errores', () => {
    expect(esErrorDeCodigo(new ApiError(400, 'password debe tener entre 8 y 72 bytes'))).toBe(false);
    expect(esErrorDeCodigo(new ApiError(429, 'x'))).toBe(false);
    expect(esErrorDeCodigo(new ErrorRed())).toBe(false);
  });
});

describe('mensajeErrorAuth', () => {
  it('avisa de credenciales incorrectas en el login', () => {
    expect(mensajeErrorAuth(new ApiError(401, 'x'), 'login')).toBe('Correo o contraseña incorrectos.');
  });

  it('avisa de correo duplicado en el registro', () => {
    expect(mensajeErrorAuth(new ApiError(409, 'x'), 'registro')).toBe('Ese correo ya está registrado.');
  });

  it('muestra el mensaje del backend en un 400', () => {
    expect(mensajeErrorAuth(new ApiError(400, 'password debe tener entre 8 y 72 bytes'), 'registro')).toBe(
      'password debe tener entre 8 y 72 bytes'
    );
  });

  it('distingue la falta de conexión', () => {
    expect(mensajeErrorAuth(new ErrorRed(), 'login')).toMatch(/conexión/);
  });

  it('avisa del límite de intentos (429)', () => {
    expect(mensajeErrorAuth(new ApiError(429, 'x'), 'recuperacion')).toMatch(/demasiados intentos/);
  });

  it('traduce el código inválido o vencido de la recuperación', () => {
    expect(mensajeErrorAuth(new ApiError(400, 'código inválido o expirado'), 'recuperacion')).toBe(
      'El código es incorrecto o ya venció. Revísalo o pide uno nuevo.'
    );
  });

  it('en la recuperación, el 400 de la contraseña muestra el mensaje del backend', () => {
    expect(mensajeErrorAuth(new ApiError(400, 'password debe tener entre 8 y 72 bytes'), 'recuperacion')).toBe(
      'password debe tener entre 8 y 72 bytes'
    );
  });

  it('usa un mensaje genérico para lo demás', () => {
    expect(mensajeErrorAuth(new ApiError(500, 'boom'), 'login')).toBe('Algo salió mal. Inténtalo de nuevo.');
  });
});
