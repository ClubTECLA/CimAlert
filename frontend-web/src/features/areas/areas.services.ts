import type { Polygon } from 'geojson'
import type { ApiBuscarAreaRequest, ApiBuscarAreaResponse } from '@/types/api-types'
import { apiRequest, sessionToken } from '@/lib/http'

// POST /areas/buscar: el backend filtra por geometría con PostGIS. Los filtros de categoría, estado o capacidad
// se siguen aplicando en el cliente sobre este subconjunto (reportFilters y centroFormat).

// Reportes y centros piden el mismo área al cargar. Mientras la petición esté en vuelo, comparten la respuesta:
// el mapa hace una sola llamada al backend por dibujo en vez de dos idénticas. La sesión entra en la clave: un
// token nuevo no puede reaprovechar la petición de la sesión anterior.
let inflight: Promise<ApiBuscarAreaResponse> | null = null
let inflightKey = ''

export function buscarArea(poligono: Polygon): Promise<ApiBuscarAreaResponse> {
    const key = JSON.stringify([sessionToken.get(), poligono])
    if (inflight && inflightKey === key) return inflight

    const body: ApiBuscarAreaRequest = {
        poligono: {
            type: 'Polygon',
            coordinates: poligono.coordinates,
        },
    }
    const request: Promise<ApiBuscarAreaResponse> = apiRequest('/areas/buscar', { method: 'POST', json: body })
    inflight = request
    inflightKey = key
    // then con dos ramas en vez de finally: así la promesa de limpieza nunca queda rechazada sin manejar.
    void request.then(limpiar, limpiar)

    function limpiar() {
        if (inflight === request) {
            inflight = null
            inflightKey = ''
        }
    }

    return request
}

/** Fuerza una consulta nueva: la usan las altas que necesitan ver su propio cambio, no las lecturas concurrentes. */
export function invalidarBuscarArea() {
    inflight = null
    inflightKey = ''
}
