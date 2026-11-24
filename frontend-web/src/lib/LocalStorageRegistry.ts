import { AREA_FILTER_STORAGE_KEY, type AreaFilterPreferencesType, type GeneralPreferencesType, type localStorageRegistryType, type SearchStorageType, type SecurityPreferencesType } from "@/types/localStorage-types";

export const GeneralPreferencesBase: GeneralPreferencesType = {
    'dark_mode' : false,
    'show_location_in_searchBar' : false
}

export const SecurityPreferencesBase: SecurityPreferencesType = {
    'remember_in_this_device': false,
    'two_steps_verification': true
}

export const SeachStorageBase: SearchStorageType = {
    recent_searches: [],
    current_location_suggestions: [],
    selected_recent_search: "",
    current_query: "",
}

// 500 reportes × unas decenas de vértices sigue siendo trabajo de menos de un frame en un equipo modesto;
// por encima compensa mandar el polígono a PostGIS. El modo backend es el predeterminado.
export const AreaFilterPreferencesBase: AreaFilterPreferencesType = {
    mode: 'backend',
    threshold: 500,
}

export const localStorageBaseRegistry: localStorageRegistryType = {
    'Preferencias generales': GeneralPreferencesBase,
    'Preferencias de seguridad': SecurityPreferencesBase,
    'Almacenamiento de Busqueda': SeachStorageBase,
    [AREA_FILTER_STORAGE_KEY]: AreaFilterPreferencesBase,
};

