import type { PhotonFeature } from "@/lib/usePhotonSearch";

export type GeneralPreferencesType = {
    dark_mode: boolean;
    show_location_in_searchBar: boolean;
}

export type SecurityPreferencesType = {
    remember_in_this_device: boolean;
    two_steps_verification: boolean;
}

export type SearchStorageType = {
    recent_searches: string[];
    current_location_suggestions: PhotonFeature[];
    selected_recent_search: string;
    current_query: string;
}

/** 'auto' decide por el número de reportes; 'frontend' y 'backend' fuerzan el modo. */
export type AreaFilterMode = 'auto' | 'frontend' | 'backend';

export type AreaFilterPreferencesType = {
    mode: AreaFilterMode;
    /** Reportes a partir de los cuales 'auto' pasa a filtrar en el backend. */
    threshold: number;
}

export const AREA_FILTER_STORAGE_KEY = 'Preferencias de filtrado por área';

export type localStorageRegistryType = {
    "Preferencias generales": GeneralPreferencesType;
    "Preferencias de seguridad": SecurityPreferencesType;
    "Almacenamiento de Busqueda": SearchStorageType;
    [AREA_FILTER_STORAGE_KEY]: AreaFilterPreferencesType;
};

export type localStorageKeys = keyof localStorageRegistryType;

export type localStorageValues<K extends localStorageKeys> = localStorageRegistryType[K];