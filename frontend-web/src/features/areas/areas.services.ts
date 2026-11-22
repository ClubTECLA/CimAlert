import type { Polygon } from 'geojson'
import type { ApiBuscarAreaRequest, ApiBuscarAreaResponse } from '@/types/api-types'
import { apiRequest } from '@/lib/http'

// POST /areas/buscar: el backend filtra por geometría con PostGIS. Los filtros de categoría, estado o capacidad
// se siguen aplicando en el cliente sobre este subconjunto (reportFilters y centroFormat).
export function buscarArea(poligono: Polygon): Promise<ApiBuscarAreaResponse> {
    const body: ApiBuscarAreaRequest = {
        poligono: {
            type: 'Polygon',
            coordinates: poligono.coordinates,
        },
    }
    return apiRequest('/areas/buscar', { method: 'POST', json: body })
}
