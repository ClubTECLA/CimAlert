// Pruebas de mappers.ts: la traducción de las respuestas del backend (shared/openapi) a los tipos
// que usan las pantallas de mobile. Si el backend cambia un campo o un estado, aquí se nota antes
// que en la app.
import { describe, expect, it } from 'vitest';
import {
  categoriaDeIncidente,
  autorDeComentario,
  esRolOficial,
  mapearComentario,
  mapearDetalle,
  mapearEstado,
  mapearResumen,
  zonasActivas,
  origenDeApi,
  urlMultimedia,
  type IncidenteApi,
  type ReporteApi,
  type ReporteResumenApi,
} from './mappers';

// Fecha fija para que el `created_at` de respaldo de mapearResumen sea predecible.
const AHORA = new Date('2026-10-22T12:00:00.000Z');

// Arma una entrada del catálogo `GET /incidentes`; solo importan id, nombre (slug) y etiqueta.
const incidente = (id: number, nombre: string, etiqueta = nombre): IncidenteApi => ({
  id,
  nombre,
  etiqueta,
  tiempo_limite: 5,
  radio: 100,
  color: '#000000',
  tags: [],
  esta_activo: true,
});

const catalogo = [incidente(1, 'luz'), incidente(2, 'socavon'), incidente(7, 'incendio', 'Incendio')];

// Un reporte como llega en la lista (`GET /reportes/cercanos`); cada test cambia solo lo que prueba.
const resumen = (parcial: Partial<ReporteResumenApi> = {}): ReporteResumenApi => ({
  id: 'r1',
  incidente_id: 2,
  titulo: 'Socavón en Calle X',
  direccion: 'Calle X',
  estado_actual: 'Pendiente',
  avistamientos: 4,
  es_oficial: false,
  latitude: 32.5,
  longitude: -117.01,
  ...parcial,
});

// El backend tiene 5 estados y mobile muestra 3. Comprueba cada equivalencia y que lo que no se
// debe mostrar (Descartado o un estado nuevo que mobile no conozca) quede fuera del mapa.
describe('mapearEstado', () => {
  it.each([
    ['Pendiente', 'pendiente'],
    ['Probable', 'pendiente'],
    ['Verificado', 'en_proceso'],
    ['Resuelto', 'resuelto'],
  ])('%s -> %s', (backend, esperado) => {
    expect(mapearEstado(backend)).toBe(esperado);
  });

  it('Descartado y valores desconocidos no tienen estado (se excluyen)', () => {
    expect(mapearEstado('Descartado')).toBeNull();
    expect(mapearEstado('Otro')).toBeNull();
  });
});

// La categoría (icono y filtro del mapa) sale del slug del incidente. Comprueba que una categoría
// nueva del backend funcione sin cambiar mobile, y que un incidente desconocido no rompa la lista.
describe('categoriaDeIncidente', () => {
  it('usa el nombre (slug) del incidente', () => {
    expect(categoriaDeIncidente(incidente(2, 'socavon'))).toBe('socavon');
  });

  it('una categoría que mobile no tenía hardcodeada (incendio) es su propia categoría', () => {
    expect(categoriaDeIncidente(incidente(7, 'incendio', 'Incendio'))).toBe('incendio');
  });

  it('un incidente ausente del catálogo cae en "otro"', () => {
    expect(categoriaDeIncidente(undefined)).toBe('otro');
  });
});

// Cómo se firma un comentario: el personal (cualquier rol que no sea ciudadano) firma con el nombre
// de su rol y el ciudadano queda anónimo. También cubre roles nuevos o raros, para que nunca se
// muestre algo vacío ni se rompa con nombres que existen en Object ("constructor").
describe('esRolOficial / autorDeComentario', () => {
  it('todo rol distinto de ciudadano es oficial y se firma con el nombre del rol', () => {
    expect(esRolOficial('rescatista')).toBe(true);
    expect(autorDeComentario('rescatista')).toBe('Rescatista');
    expect(autorDeComentario('proteccion_civil')).toBe('Protección Civil');
    expect(autorDeComentario('admin')).toBe('Administrador');
    expect(autorDeComentario('moderador')).toBe('Moderador');
    expect(autorDeComentario('analista')).toBe('Analista');
  });

  it('un rol desconocido se muestra tal cual', () => {
    expect(autorDeComentario('inspector')).toBe('inspector');
  });

  it('un rol que coincide con una propiedad heredada de Object sigue siendo texto', () => {
    expect(autorDeComentario('constructor')).toBe('constructor');
    expect(autorDeComentario('toString')).toBe('toString');
  });

  it('ciudadano y rol ausente son "Anónimo"', () => {
    expect(esRolOficial('ciudadano')).toBe(false);
    expect(esRolOficial(undefined)).toBe(false);
    expect(esRolOficial('')).toBe(false);
    expect(autorDeComentario('')).toBe('Anónimo');
    expect(autorDeComentario('  ')).toBe('Anónimo');
    expect(autorDeComentario('ciudadano')).toBe('Anónimo');
    expect(autorDeComentario(undefined)).toBe('Anónimo');
  });
});

// Las fotos las sirve nginx en /multimedia, fuera de /api/v1. Comprueba que la URL de la foto se
// arme con el host de la API (con o sin puerto) y sin el prefijo de la API.
describe('origenDeApi / urlMultimedia', () => {
  it('quita la ruta (/api/v1) y deja esquema + host + puerto', () => {
    expect(origenDeApi('http://10.0.2.2/api/v1')).toBe('http://10.0.2.2');
    expect(origenDeApi('https://api.ejemplo.mx:8443/api/v1/')).toBe('https://api.ejemplo.mx:8443');
  });

  it('arma la URL pública de la foto sin el prefijo /api', () => {
    expect(urlMultimedia('abc/foto.jpg', 'http://10.0.2.2/api/v1')).toBe(
      'http://10.0.2.2/multimedia/abc/foto.jpg',
    );
  });

  it('tolera una barra inicial en el path', () => {
    expect(urlMultimedia('/foto.jpg', 'http://10.0.2.2/api/v1')).toBe('http://10.0.2.2/multimedia/foto.jpg');
  });
});

// Cada elemento de la lista del mapa: comprueba campo por campo la traducción y que un reporte
// Descartado no llegue al mapa.
describe('mapearResumen', () => {
  it('traduce campos del backend al tipo Reporte de mobile', () => {
    expect(mapearResumen(resumen(), catalogo, AHORA)).toEqual({
      id: 'r1',
      titulo: 'Socavón en Calle X',
      categorias: ['socavon'],
      tags: [],
      lat: 32.5,
      lng: -117.01,
      direccion: 'Calle X',
      image_url: undefined,
      status: 'pendiente',
      upvotes: 4,
      user_id: '',
      // El resumen no trae fecha: se usa "ahora" (el período se filtra en servidor con `desde`).
      created_at: AHORA.toISOString(),
    });
  });

  it('devuelve null para un reporte Descartado', () => {
    expect(mapearResumen(resumen({ estado_actual: 'Descartado' }), catalogo, AHORA)).toBeNull();
  });
});

// Cada comentario del hilo: el autor sale del rol, no del username, para que un ciudadano nunca
// quede expuesto aunque el backend mande su nombre.
describe('mapearComentario', () => {
  it('un rol oficial firma con el nombre del rol, no con el username', () => {
    const c = mapearComentario({
      id: 9,
      reporte_id: 'r1',
      user_id: 'u1',
      username: 'jose_m',
      rol_name: 'rescatista',
      comentario: 'En camino',
      created_at: '2026-10-22T10:00:00Z',
    });
    expect(c).toEqual({
      id: 9,
      reporte_id: 'r1',
      user_id: 'u1',
      autor: 'Rescatista',
      es_oficial: true,
      comentario: 'En camino',
      created_at: '2026-10-22T10:00:00Z',
    });
  });

  it('un ciudadano es "Anónimo" aunque el backend mande su username', () => {
    const c = mapearComentario({
      id: 1,
      reporte_id: 'r1',
      user_id: 'u1',
      username: 'maria_g',
      rol_name: 'ciudadano',
      comentario: 'x',
      created_at: '2026-10-22T10:00:00Z',
    });
    expect(c.autor).toBe('Anónimo');
    expect(c.es_oficial).toBe(false);
  });
});

// La pantalla de detalle: a diferencia de la lista, trae fecha real, etiquetas y foto. Comprueba de
// dónde sale cada una y el caso de un Descartado abierto por enlace directo.
describe('mapearDetalle', () => {
  const detalle = (parcial: Partial<ReporteApi> = {}): ReporteApi => ({
    ...resumen(),
    es_historico: false,
    created_at: '2026-10-22T08:00:00Z',
    updated_at: '2026-10-22T09:00:00Z',
    comentarios: [],
    multimedia: [{ id: 1, path: 'a/b.jpg', user_id: 'u1', created_at: '2026-10-22T08:00:00Z' }],
    tags: [
      { id: 1, nombre: 'Profundo', count: 3 },
      { id: 2, nombre: 'Peligroso', count: 1 },
    ],
    locations: [],
    ...parcial,
  });

  it('toma fecha real, tags por nombre y la primera foto como image_url', () => {
    const r = mapearDetalle(detalle(), catalogo, 'http://10.0.2.2/api/v1');
    expect(r.created_at).toBe('2026-10-22T08:00:00Z');
    expect(r.tags).toEqual(['Profundo', 'Peligroso']);
    expect(r.image_url).toBe('http://10.0.2.2/multimedia/a/b.jpg');
    expect(r.categorias).toEqual(['socavon']);
    expect(r.upvotes).toBe(4);
  });

  it('sin multimedia no hay image_url', () => {
    expect(mapearDetalle(detalle({ multimedia: [] }), catalogo, 'http://10.0.2.2/api/v1').image_url).toBeUndefined();
  });

  it('un detalle Descartado se muestra como pendiente (solo se llega por id directo)', () => {
    const r = mapearDetalle(detalle({ estado_actual: 'Descartado' }), catalogo, 'http://10.0.2.2/api/v1');
    expect(r.status).toBe('pendiente');
  });
});

// Las zonas que vigilan las alertas cercanas: un reporte resuelto sigue en el mapa, pero ya no debe
// generar alertas.
describe('zonasActivas', () => {
  const base = { titulo: 'X', categorias: [], tags: [], lat: 32.5, lng: -117, upvotes: 0, user_id: 'u1', created_at: '' };

  it('descarta los resueltos y deja solo id, título y coordenadas', () => {
    const zonas = zonasActivas([
      { ...base, id: 'a', status: 'pendiente' },
      { ...base, id: 'b', status: 'resuelto' },
      { ...base, id: 'c', status: 'en_proceso' },
    ]);
    expect(zonas).toEqual([
      { id: 'a', titulo: 'X', lat: 32.5, lng: -117 },
      { id: 'c', titulo: 'X', lat: 32.5, lng: -117 },
    ]);
  });
});
