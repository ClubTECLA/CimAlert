import { useCallback, useRef } from 'react';
import { useFocusEffect } from 'expo-router';
import { crearVigencia, type Vigencia } from '@/lib/vigencia';

/**
 * `crearVigencia` atada al foco de la pantalla: al salir se invalidan las peticiones en curso y se
 * llama `alSalir` (p. ej. para quitar el estado de carga y que al volver los botones no sigan bloqueados).
 */
export function useVigenciaDePantalla(alSalir?: () => void): Vigencia {
  const vigencia = useRef(crearVigencia()).current;
  const alSalirRef = useRef(alSalir);
  alSalirRef.current = alSalir;
  useFocusEffect(
    useCallback(
      () => () => {
        vigencia.invalidar();
        alSalirRef.current?.();
      },
      [vigencia],
    ),
  );
  return vigencia;
}
