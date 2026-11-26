import { createContext, useContext } from 'react';

/** Espacio que queda entre el campo enfocado y el borde superior del teclado. */
export const MARGEN_SOBRE_TECLADO = 24;

/**
 * Cuánto hay que bajar el scroll para que el campo quede arriba del teclado (0: ya se ve).
 * Todo en coordenadas de pantalla: `campoAbajo` es el borde inferior del campo y `tecladoArriba`
 * el borde superior del teclado.
 */
export function desplazamientoParaVer(campoAbajo: number, tecladoArriba: number, margen = MARGEN_SOBRE_TECLADO): number {
  return Math.max(0, campoAbajo - (tecladoArriba - margen));
}

/** Lo pone AuthScreenShell: recorre la pantalla hasta dejar visible el campo con el foco. */
export const TecladoContext = createContext<{ mostrarCampoEnfocado: () => void }>({
  mostrarCampoEnfocado: () => {},
});

/** Para el `onFocus` de un campo dentro de AuthScreenShell; respeta el `onFocus` que ya tuviera. */
export function useAlEnfocar<E>(onFocus?: (e: E) => void): (e: E) => void {
  const { mostrarCampoEnfocado } = useContext(TecladoContext);
  return (e: E) => {
    onFocus?.(e);
    mostrarCampoEnfocado();
  };
}
