import { describe, expect, it } from 'vitest';
import { crearVigencia } from './vigencia';

// La respuesta de una petición solo debe navegar o mostrar alertas si el usuario sigue en la pantalla
// donde la pidió (recuperar contraseña, nueva contraseña, reenviar código).
describe('crearVigencia', () => {
  it('una petición sigue vigente mientras no se salga de la pantalla', () => {
    const v = crearVigencia();
    expect(v.vigente(v.iniciar())).toBe(true);
  });

  it('al salir de la pantalla, la petición en curso deja de estar vigente', () => {
    const v = crearVigencia();
    const marca = v.iniciar();
    v.invalidar();
    expect(v.vigente(marca)).toBe(false);
  });

  it('volver a la pantalla no revive una petición vieja, pero sí valen las nuevas', () => {
    const v = crearVigencia();
    const vieja = v.iniciar();
    v.invalidar(); // salió y volvió
    const nueva = v.iniciar();
    expect(v.vigente(vieja)).toBe(false);
    expect(v.vigente(nueva)).toBe(true);
  });
});
