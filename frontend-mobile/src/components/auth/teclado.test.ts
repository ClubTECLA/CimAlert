import { describe, expect, it } from 'vitest';
import { desplazamientoParaVer } from './teclado';

// Pantalla de ejemplo: el teclado empieza en y = 2000.
const TECLADO_ARRIBA = 2000;

describe('desplazamientoParaVer', () => {
  it('un campo que ya se ve arriba del teclado (con margen) no mueve el scroll', () => {
    expect(desplazamientoParaVer(1900, TECLADO_ARRIBA, 24)).toBe(0);
  });

  it('un campo tapado por el teclado sube justo hasta quedar el margen arriba de él', () => {
    // Borde inferior en 2300: tiene que subir 300 para llegar a 2000 y 24 más de margen.
    expect(desplazamientoParaVer(2300, TECLADO_ARRIBA, 24)).toBe(324);
  });

  it('un campo pegado al teclado, sin margen, sube solo lo del margen', () => {
    expect(desplazamientoParaVer(2000, TECLADO_ARRIBA, 24)).toBe(24);
  });
});
