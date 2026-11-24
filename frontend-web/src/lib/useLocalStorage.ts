import {
  useState,
  useEffect,
  useCallback,
  useId,
  type SetStateAction,
} from 'react';
import type { localStorageKeys, localStorageValues } from '@/types/localStorage-types';
import { localStorageBaseRegistry } from '@/lib/LocalStorageRegistry';

/** Nombre del evento interno que notifica cambios de preferencias en esta pestaña. */
export const LOCAL_STORAGE_CHANGE_EVENT = 'cimAlert:local-storage-change';

/** Datos compartidos al sincronizar una clave entre instancias del hook en la misma pestaña. */
export type LocalStorageChangeDetail = {
  /** Clave del registro de preferencias que cambió. */
  key: string;
  /** Valor serializado nuevo, o `null` si la clave se eliminó. */
  value: string | null;
  /** Identificador de la instancia que emitió el evento para ignorar auto-notificaciones. */
  sourceId: string;
};

/**
 * Convierte el valor guardado desde JSON al tipo asociado a la clave.
 * Si no hay valor o el JSON está dañado, devuelve el valor predeterminado del registro.
 */
function parseValue<K extends localStorageKeys>(key: K, item: string | null): localStorageValues<K> {
  if (item === null) return localStorageBaseRegistry[key];

  try {
    const parsed: unknown = JSON.parse(item);
    if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return localStorageBaseRegistry[key];
    }
    return { ...localStorageBaseRegistry[key], ...parsed } as localStorageValues<K>;
  } catch (error) {
    console.error(`Error reading localStorage for the key "${key}":`, error);
    return localStorageBaseRegistry[key];
  }
}

/**
 * Lee una clave del almacenamiento y usa su valor predeterminado si no se puede leer.
 * También lo usan los providers de reportes y centros: no son componentes y no pueden llamar al hook.
 */
export function readStoredValue<K extends localStorageKeys>(key: K): localStorageValues<K> {
  try {
    return parseValue(key, localStorage.getItem(key));
  } catch (error) {
    console.error(`Error reading localStorage for the key "${key}":`, error);
    return localStorageBaseRegistry[key];
  }
}

/** Identifica los actualizadores funcionales aceptados por los setters de React. */
function isStateUpdater<T>(value: SetStateAction<T>): value is (previousValue: T) => T {
  return typeof value === 'function';
}

/**
 * Base de una actualización funcional: storage, no una copia en memoria ni el valor capturado por un render. Cada
 * escritura exitosa ya guardó en storage de forma sincrónica, así que releer cubre escrituras posteriores de otro
 * effect, de otra instancia del hook y cambios de clave de ida y vuelta sin estado intermedio.
 */
export function applyStoredUpdate<K extends localStorageKeys>(
  key: K,
  action: SetStateAction<localStorageValues<K>>,
): localStorageValues<K> {
  const stored = readStoredValue(key);
  return isStateUpdater(action) ? action(stored) : action;
}

/** Dice si el valor que vio el render difiere de storage; se usa al suscribirse para recuperar eventos perdidos. */
export function storageChanged<K extends localStorageKeys>(
  rendered: localStorageValues<K>,
  stored: localStorageValues<K>,
): boolean {
  return JSON.stringify(rendered) !== JSON.stringify(stored);
}

/**
 * Mantiene una preferencia sincronizada con `localStorage`.
 *
 * La clave determina el tipo del valor y su valor predeterminado, definidos en
 * `localStorage-types` y `LocalStorageRegistry`. Los cambios se guardan y se
 * notifican a otras instancias del hook en esta pestaña; los cambios hechos en
 * otra pestaña se reciben mediante el evento nativo `storage`.
 *
 * @typeParam K Clave registrada en `localStorageRegistryType`.
 * @param key Clave de preferencia que se leerá y actualizará.
 * @returns Una tupla con el valor actual, un setter que devuelve si la escritura funcionó
 * y una función que elimina la clave y restablece el estado al valor predeterminado.
 *
 * @example
 * const [preferencias, setPreferencias, borrarPreferencias] =
 *   useLocalStorage('Preferencias generales');
 *
 * if (!setPreferencias((actuales) => ({ ...actuales, dark_mode: true }))) {
 *   // La escritura falló: el valor guardado sigue siendo el anterior.
 * }
 * borrarPreferencias();
 */
export function useLocalStorage<K extends localStorageKeys>(key: K) {
  const sourceId = useId();
  const [valueForKey, setValueForKey] = useState(() => ({
    key,
    value: readStoredValue(key),
  }));

  // Al cambiar de clave se descarta el valor de la anterior: al volver debe leerse storage, porque mientras se estaba
  // suscrito a otra clave no se recibieron sus cambios. React admite ajustar estado durante el render por una prop.
  const storedValue = valueForKey.key === key ? valueForKey.value : readStoredValue(key);
  if (valueForKey.key !== key) {
    setValueForKey({ key, value: storedValue });
  }

  const applyExternalValue = useCallback((value: string | null) => {
    setValueForKey({ key, value: parseValue(key, value) });
  }, [key]);

  const setStoredValue = useCallback(
    (action: SetStateAction<localStorageValues<K>>) => {
      const next = applyStoredUpdate(key, action);

      let serialized: string;
      try {
        serialized = JSON.stringify(next);
        localStorage.setItem(key, serialized);
      } catch (error) {
        console.error(`Error saving the key "${key}" in localStorage :`, error);
        // El estado no se toca: el valor guardado sigue siendo el anterior para poder reintentar.
        return false;
      }

      setValueForKey({ key, value: next });
      // El evento nativo `storage` no se dispara en la pestaña que hizo el cambio.
      window.dispatchEvent(new CustomEvent<LocalStorageChangeDetail>(LOCAL_STORAGE_CHANGE_EVENT, {
        detail: { key, value: serialized, sourceId },
      }));
      return true;
    },
    [key, sourceId],
  );

  // Sincroniza instancias del hook en esta pestaña y cambios originados en otras pestañas.
  useEffect(() => {
    const handleLocalStorageChange = (event: Event) => {
      const detail = (event as CustomEvent<LocalStorageChangeDetail>).detail;
      if (detail?.key === key && detail.sourceId !== sourceId) applyExternalValue(detail.value);
    };
    const handleStorage = (event: StorageEvent) => {
      if (event.key === key) applyExternalValue(event.newValue);
    };

    window.addEventListener(LOCAL_STORAGE_CHANGE_EVENT, handleLocalStorageChange);
    window.addEventListener('storage', handleStorage);

    // Otra instancia pudo escribir en un effect de layout antes de que esta se suscribiera: su evento síncrono se
    // perdió y el nativo no llega a la misma pestaña. storage es la fuente de verdad, así que se recupera justo
    // después de subscribirse. En microtarea para no actualizar el estado dentro del cuerpo del efecto.
    let active = true;
    queueMicrotask(() => {
      if (!active) return;
      const latest = readStoredValue(key);
      if (storageChanged(storedValue, latest)) applyExternalValue(JSON.stringify(latest));
    });

    return () => {
      active = false;
      window.removeEventListener(LOCAL_STORAGE_CHANGE_EVENT, handleLocalStorageChange);
      window.removeEventListener('storage', handleStorage);
    };
  }, [key, sourceId, applyExternalValue, storedValue]);

  /** Elimina la clave y actualiza las instancias al valor predeterminado sin volver a guardarlo. */
  const removeStoredValue = useCallback(() => {
    try {
      localStorage.removeItem(key);
    } catch (error) {
      console.error(`Error removing the key "${key}" from localStorage:`, error);
      return false;
    }

    window.dispatchEvent(new CustomEvent<LocalStorageChangeDetail>(LOCAL_STORAGE_CHANGE_EVENT, {
      detail: { key, value: null, sourceId },
    }));
    setValueForKey({ key, value: localStorageBaseRegistry[key] });
    return true;
  }, [key, sourceId]);

  return [storedValue, setStoredValue, removeStoredValue] as const;
}
