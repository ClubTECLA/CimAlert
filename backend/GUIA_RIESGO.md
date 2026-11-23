# Guía: modelo de riesgo

Esta guía explica de dónde salen los datos de la funcionalidad de riesgo (pantalla
"Riesgo e histórico" del panel web), cómo se calcula el peso de cada reporte y qué
falta definir. El código vive en `internal/riesgo/`; los datos, en la vista
`riesgo_historico` (migración `003_riesgo_datasets`, que reemplaza a la vista
`reporte_historico_peso` de la `002_riesgo`).

**Versión del modelo:** 2 (la 1 solo leía reportes; la 2 agrega los datasets importados).

---

## Qué es y qué no es

El riesgo muestra **cómo se han comportado los incidentes a lo largo del tiempo**:
dónde se concentran, en qué colonias y de qué tipo. Responde a los requerimientos
F-4 (zonas más propensas a tener incidentes, basadas en la información histórica) y
+6 (un riesgo calculado no se presenta como un incidente confirmado).

| | Incidentes | Riesgo |
|---|---|---|
| Datos | Reportes activos (`NOT es_historico`) | Reportes históricos (`es_historico`) y datasets importados |
| Pregunta | ¿Qué está pasando ahora? | ¿Dónde y qué ha pasado más? |
| Tiempo | En vivo (sondeo) | Periodo elegido (`desde` / `hasta`) |

Por eso un reporte nunca aparece en las dos: mientras está activo es de Incidentes y,
cuando se archiva como histórico, pasa a contar para el riesgo. Las filas de un dataset
importado solo existen en el riesgo: no son reportes.

---

## Fuente de datos

Todas las consultas leen la vista `riesgo_historico`. Es la única puerta a los datos
del riesgo, así que ninguna consulta puede mezclar por error reportes activos o
descartados. Junta dos fuentes con las mismas columnas (`UNION ALL`):

1. **Reportes de la app** (`origen = 'reporte'`): un registro por reporte que cumple
   - `reporte.es_historico = true`, y
   - `estado_actual <> 'Descartado'`: el riesgo usa solo información válida
     (requerimiento R-10).
2. **Datasets importados** (`origen = 'dataset'`): un registro por fila de
   `incidente_historico`. Ver [Datasets históricos](#datasets-históricos).

| Columna | Reporte | Dataset |
|---|---|---|
| `origen` | `'reporte'` | `'dataset'` |
| `reporte_id` / `dataset_id` | el del reporte / `NULL` | `NULL` / el del dataset |
| `incidente_id` | `reporte.incidente_id` | la categoría de la fila |
| `created_at` | `reporte.created_at` | el día 1 del mes, a mediodía en Tijuana (así no cambia de mes al pasar a UTC) |
| `geom` | `puntos_origen.geom`: el centroide de las ubicaciones del reporte | el punto de la fila |
| `colonia` | La parte después de ` · ` en `reporte.direccion` (`"calle · colonia"`); `NULL` si no hay | la columna Colonia; `NULL` si va vacía |
| `personas_afectadas`, `fallecidos` | `NULL` | las columnas del archivo; `NULL` si van vacías |
| `peso` | Ver [Peso](#peso) | Ver [Peso](#peso) |

Las dos fuentes funcionan a la vez: los reportes que un rol oficial pasa a histórico y
los datasets que se importan cuentan igual en el mapa de calor, las zonas, el
histórico por colonia, el conteo por categoría y la exportación.

**Quién marca un reporte como histórico:** cualquier rol distinto de `ciudadano`, a
mano, con `PUT /reportes/{id}/historico` (y `DELETE` para devolverlo a Incidentes).
Es idempotente y queda en el `historial` (`reporte.marcar_historico` /
`reporte.desmarcar_historico`). Todavía no hay un paso automático (p. ej. al
resolverse o al vencer `tiempo_limite`); la tabla `logs` del baseline sigue sin usarse
y es la alternativa si se prefiere guardar una foto inmutable al archivar.

---

## Peso

```
reporte: peso = credibilidad(estado) × (1 + ln(1 + avistamientos))
dataset: peso = 1.0 × (1 + ln(1 + personas_afectadas))      (vacío = 0)
```

Las dos fórmulas tienen la misma forma para que las dos fuentes queden en la misma
escala. Una fila importada es un dato oficial, así que tiene la credibilidad máxima
(1.0, como un reporte verificado), y las personas afectadas hacen el papel de los
avistamientos: una fila con 0 afectados pesa 1; con 2, ≈ 2.1; con 21, ≈ 4.1; con 160,
≈ 6.1. Los fallecidos se guardan y se exportan, pero no entran al peso.

Credibilidad de un reporte:

| Estado | Credibilidad | Por qué |
|---|---|---|
| Verificado | 1.0 | Confirmado |
| Resuelto | 1.0 | Ocurrió: que ya se haya resuelto no cambia lo que dice del lugar |
| Probable | 0.75 | Tuvo confirmaciones, pero no se verificó |
| Pendiente | 0.5 | Nadie lo confirmó; aporta, con menos peso |
| Descartado | — | No entra en la vista |

- **Avistamientos con logaritmo:** el primer avistamiento sube el factor de 1 a ≈ 1.7,
  pero el décimo aporta mucho menos; así un solo reporte muy visto no tapa una zona con
  muchos reportes. Con 2 avistamientos el factor es ≈ 2.1; con 6, ≈ 2.9.
- **`es_oficial` no multiplica:** un aviso oficial nace verificado, así que ya tiene
  la credibilidad máxima.
- **Sin recencia:** un reporte de 2019 pesa lo mismo que uno de 2026. El tiempo se
  acota con el periodo que pide el cliente.
- **No usa la severidad de los tags** (`reportes_scores.total_score`): crece cuando la
  gente confirma tags, la misma señal que los avistamientos, y se contaría dos veces.

---

## Mapa de calor

`ListCeldasCalor` agrupa los reportes del área visible en una cuadrícula y devuelve,
por celda, la suma de los pesos y cuántos reportes cayeron en ella.

- La cuadrícula es en metros reales: `geom` se proyecta con `a_metros` (migración
  002) a UTM zona 11N (EPSG:32611, la de Tijuana) y `ST_SnapToGrid` lleva cada punto
  al nodo de su celda. Las zonas usan la misma función. Para otra ciudad basta
  cambiar la proyección dentro de `a_metros` (en una migración nueva).
- **Punto de cada celda:** el promedio de la posición de sus reportes, no el nodo. El
  nodo solo agrupa (la consulta lo devuelve como `celda_x`, `celda_y`, que no salen
  en la API). Como el tamaño de la celda cambia con el zoom, con el nodo el punto
  saltaba al acercarse o alejarse: un reporte solo quedaba hasta a ~2 km de su sitio
  en zoom 10. Con el promedio, un reporte solo queda siempre en su lugar y un grupo,
  en su centro.
- El filtro por área (`&&` sobre `geography`) usa el índice GiST de
  `puntos_origen.geom`. Como los bordes norte y sur de esa caja son arcos de círculo
  máximo (en el hemisferio norte se curvan hacia el polo), la consulta filtra además
  la latitud exacta `[min_lat, max_lat]`.
- **Áreas anchas:** en `geography` los bordes del rectángulo son arcos de círculo
  máximo. Con 180° o más de longitud se interpreta al revés (0 filas), y con
  exactamente 180° PostGIS falla ("Antipodal edge"). Pasa con el mapa muy alejado.
  Por eso el servicio parte el área en tramos de a lo más 90° (`PartirArea`), hace
  una consulta por tramo y junta por su nodo (`celda_x`, `celda_y`) las celdas que
  salen en dos tramos, con el punto en el promedio de los reportes de las dos partes.
  Como `&&` incluye el borde, cada tramo filtra además la longitud exacta como
  `[min, max)` (el último, `[min, max]`): un reporte justo en el borde cuenta una
  sola vez. El área de la ciudad queda en un solo tramo.
- El área llega siempre en longitudes de -180 a 180 (lo exige el contrato). El web
  convierte las de las copias del mundo que repite MapLibre al desplazarse, y si la
  vista cruza el antimeridiano pide todas las longitudes.
- **Tamaño de celda según el zoom:** unos 32 px en pantalla. A la latitud de Tijuana
  un píxel mide ≈ 132 030 / 2^zoom metros, así que la celda es
  `132 030 / 2^zoom × 32` m, limitada a [50 m, 5 km]: ≈ 4.1 km en zoom 10, 516 m en
  zoom 13 y 64 m en zoom 16. Así la densidad visual es la misma en cualquier zoom.
- `peso_maximo` es el mayor peso entre las celdas devueltas; el cliente lo usa para
  normalizar la intensidad del calor.

## Histórico por colonia

`ListColoniasHistorico` devuelve las colonias con más peso en el periodo (las 10
primeras). Los reportes sin colonia no entran en esta lista, pero sí en el mapa de
calor, las zonas y los totales. El nombre se toma tal cual de la dirección (sin
espacios sobrantes): dos escrituras distintas de la misma colonia cuentan por separado.

## Conteo por categoría

`ListConteoCategorias` cuenta los reportes por tipo de incidente; la suma es el total
de históricos del periodo. El nombre y el color de cada tipo los pone el cliente desde
el catálogo (`GET /incidentes`).

Las tres consultas del resumen (zonas, colonias y categorías) corren en una
transacción de solo lectura `REPEATABLE READ` (`Store.LeerMismaFoto`): todas ven los
mismos datos, así que un reporte que pasa a histórico a mitad de la petición no deja
una zona con más reportes que `total_historicos`.

## Zonas de riesgo

`ListZonasRiesgo` agrupa los reportes históricos cercanos entre sí con DBSCAN
(`ST_ClusterDBSCAN`, sobre UTM 11N para trabajar en metros). Un reporte forma zona si
tiene al menos `min_puntos` reportes (contándose a sí mismo) a menos de `eps_m` metros;
los reportes aislados quedan como ruido y no forman zona, pero sí cuentan en el mapa de
calor, el histórico y los totales.

| Parámetro | Valor | Por qué |
|---|---|---|
| `eps_m` | 500 m | Del tamaño de una colonia (radios de 180 a 600 m con los datos reales). Con 1 km las zonas se encadenan (hasta 3 km de radio). |
| `min_puntos` | 4 | Cuatro incidentes cercanos ya son un patrón; con 3 se cuentan como zona repeticiones del mismo punto. |

Los dos llegan como parámetros de la consulta (constantes `EpsM` y `MinPuntos` del
servicio), así que se ajustan sin migración. Se calibraron con los 415 históricos
reales (lluvias de Baja California):

| eps / mín. | Zonas | Reportes sueltos |
|---|---|---|
| 300 m / 5 | 12 | 322 |
| **500 m / 4** | 25 | 234 |
| 300 m / 3 | 37 | 240 |
| 1000 m / 5 | 19 | 184 (zonas de hasta 3 km) |

Se devuelven las 20 zonas de más peso.

Por cada zona se devuelve:

- **Centro:** el centroide de sus reportes.
- **Radio:** el percentil 90 de la distancia de sus reportes al centro. El círculo
  cubre dónde se concentran; un reporte aislado en el borde no lo infla.
- **Colonia:** la más frecuente entre sus reportes. Si ninguno tiene colonia, la
  consulta devuelve `''` y la zona se nombra "Zona sin colonia": sigue siendo riesgo
  real y no se oculta.
- **Reportes y peso:** cuántos reportes y la suma de sus pesos. El orden es por peso.

DBSCAN corre sobre todos los históricos del periodo en cada consulta del resumen. Con
miles de reportes es rápido; si crece mucho, es el lugar para precalcular las zonas.

### Círculos o polígonos

El Figma dibuja las zonas como círculos y así se implementan. La alternativa es
devolver el contorno de la zona como polígono:
`ST_Buffer(ST_ConcaveHull(puntos, 0.4), 60)` simplificado con
`ST_SimplifyPreserveTopology`, en GeoJSON. Con los datos de prueba cada polígono
tiene entre 10 y 24 vértices (menos de 1 KB). Sigue mejor la forma real de una zona
alargada (un canal, una avenida), pero requiere agregar un campo de geometría al
contrato y dibujarlo como capa `fill`/`line` en el mapa.

## Score y nivel

El `score` de una zona o colonia es su peso dividido entre el mayor peso de la lista
devuelta (0 a 1, redondeado a 4 decimales). Es relativo: la zona más pesada del
periodo y filtro elegidos siempre vale 1.

| Nivel | Score |
|---|---|
| Alto | ≥ 0.66 |
| Medio | ≥ 0.33 |
| Bajo | < 0.33 |

`zonas_alto_riesgo` cuenta las zonas en Alto. Al ser relativo se ajusta solo a
cualquier filtro; la contra es que siempre habrá algo en Alto aunque haya pocos datos
(por eso el panel muestra también el número de reportes).

## Datasets históricos

Un dataset es un archivo `.xlsx` o `.csv` de incidentes históricos (por ejemplo, la
hoja "Formato para importación"). Tablas (migración `003`):

- `dataset_historico`: nombre, archivo original, SHA-256 del contenido (`UNIQUE`: el
  mismo archivo no se importa dos veces), filas, quién lo cargó y cuándo.
- `incidente_historico`: una fila del archivo, con su renglón original (`fila`),
  categoría, colonia, fecha (día 1 del mes), punto y personas afectadas / fallecidos.
  Índice GiST en `geom`. Se borra con su dataset (`ON DELETE CASCADE`).

| Endpoint | Qué hace |
|---|---|
| `POST /riesgo/datasets` | Importa un archivo (multipart: `file` y, opcional, `nombre`). 201 con el dataset. |
| `GET /riesgo/datasets` | Lista los datasets, del más reciente al más antiguo. |
| `DELETE /riesgo/datasets/{id}` | Borra el dataset y sus filas. 204. |
| `GET /riesgo/exportar` | Descarga los históricos de las dos fuentes (ver [Exportar](#exportar)). |

### Formato de importación

Una fila de encabezados y un incidente por fila. No importan el orden, las
mayúsculas ni los acentos, y las columnas de más se ignoran (así una hoja con un
conteo con fórmulas a un lado se importa igual).

| Columna | Obligatoria | Qué acepta |
|---|---|---|
| Latitud, Longitud | Sí | Número (punto o coma decimal), -90 a 90 y -180 a 180. El punto debe caer dentro de la zona permitida, la misma que para crear reportes (`REPORTE_POLIGONO`, por defecto Baja California); si no, el error va con la columna vacía, porque el problema es la pareja (suele ser la longitud sin el signo menos o un punto de otro estado) |
| Mes/año del Incidente (o Fecha) | Sí | Fecha de Excel, `AAAA-MM`, `AAAA-MM-DD`, `MM/AAAA` o `DD/MM/AAAA`; desde 1900 y no posterior al mes actual. Solo se guarda el mes |
| Categoria | Sí | Una categoría del catálogo, por nombre o etiqueta ("Inundación", "inundacion", "INUNDACION"). "Derrumbe" cuenta como deslave. Si el texto coincide con dos categorías (la etiqueta no es única en la base), la fila se rechaza |
| Colonia | No | Texto, hasta 200 caracteres; vacía = sin colonia |
| Personas afectadas (total), Fallecidos | No | Entero de 0 a 1,000,000; 0 es un dato y vacío = sin dato |

- **.xlsx:** se lee la primera hoja que tenga los encabezados.
- **.csv:** separador coma, punto y coma o tabulador (el que más aparezca en el
  encabezado); UTF-8, con o sin BOM, o Windows-1252 (lo que guarda Excel en Windows).
- El formato se reconoce por el contenido, no por la extensión.

**Todo o nada:** primero se valida el archivo completo. Si alguna fila es inválida no se
importa nada y la respuesta es 422 con cada error (`fila`, `columna`, `mensaje`; los
primeros 100 y `total_errores`). Así no queda un dataset a medias que haya que
limpiar. Los renglones vacíos se saltan.

**Categorías:** el importador solo usa las que ya existen en el catálogo; no crea
ninguna. Si un archivo trae una categoría nueva (como "Otro"), esa fila se rechaza; si
se quiere usar, un admin la crea con `POST /incidentes` y el importador la reconoce sola.

**Límites:** el body es de 1 MiB (el límite general de la API: ningún cambio en nginx ni
en el router), hasta 50 000 filas, y un `.xlsx` puede ocupar hasta 64 MiB descomprimido.
El archivo real de 72 filas pesa 53 KB.

Importar y borrar quedan en el `historial` (`riesgo.importar_dataset`,
`riesgo.eliminar_dataset`, entidad `dataset_historico`) con el nombre, el archivo y
las filas.

## Exportar

`GET /riesgo/exportar?formato=csv|xlsx` (CSV por defecto) con el mismo filtro que el
resumen (`incidente_id`, `desde`, `hasta`). Devuelve las dos fuentes, ordenadas por
fecha, con las columnas del formato de importación más **Origen** (`reporte` o
`dataset`) y **Dataset** (su nombre). Así lo exportado se puede volver a importar:
esas dos columnas se ignoran al importar.

- La categoría va con su etiqueta del catálogo.
- **CSV:** UTF-8 con BOM (para que Excel respete los acentos), fecha `AAAA-MM-DD`. Un
  texto que empieza con `=`, `+`, `-` o `@` sale con `'` adelante, para que Excel no
  lo tome como fórmula (la colonia sale de la dirección que escribe el ciudadano). Un
  texto que ya empieza con `'` también lleva otro adelante (`'-Norte` sale como
  `''-Norte`). Al importar un CSV se quita ese primer `'`, así que el texto vuelve
  igual; un `'Texto` escrito a mano no se toca. En el `.xlsx` no se agrega ni se quita
  nada.
- **XLSX:** hoja "Formato para importación", fecha como fecha de Excel (`mm/aaaa`).
  Excel guarda 15 dígitos significativos, así que las coordenadas pierden menos de un
  micrómetro.
- Nombre del archivo: `riesgo-historico-AAAA-MM-DD.csv` (o `.xlsx`), en
  `Content-Disposition`.
- La fecha va en hora de Tijuana con su horario de verano (la base de zonas horarias va
  embebida en el binario, porque la imagen de Alpine no la trae).

**Volver a importar lo exportado:**

- Sigue los mismos límites que cualquier importación: 50,000 filas y 1 MiB por
  archivo. Una exportación más grande se parte por periodo o por categoría. Si aun así
  más de 50,000 filas comparten mes y categoría, ningún filtro las separa: se parte el
  archivo por renglones, y cada parte conserva la fila de encabezado y queda dentro de
  los límites.
- La categoría sale con su etiqueta. Si dos categorías del catálogo comparten etiqueta
  (o el nombre de una es la etiqueta de otra), la importación rechaza esas filas por
  ambiguas. Antes de reimportar, se corrige el catálogo o se cambia la columna
  Categoria por un valor que solo coincida con una categoría (el nombre, si no choca
  con otra etiqueta).
- Se **suma** a lo que ya hay: si los datasets originales siguen cargados, sus filas
  quedan duplicadas. El sha256 compara archivos, no incidentes, y Origen y Dataset no
  identifican a un incidente ni reemplazan nada. Para reemplazar, primero se importa
  el archivo nuevo y se comprueba que funcionó (responde `201` y el dataset aparece en
  la lista con las filas esperadas); después se borra el dataset original. Así, si la
  importación falla, el original sigue cargado.

## Quién lo ve

Todos los endpoints de riesgo (`mapa-calor`, `resumen`, `datasets` y `exportar`) son
para todos los roles menos `ciudadano`: admin, moderador, analista, proteccion_civil y
rescatista. Importar y borrar datasets también.

---

## Cómo cambiar el modelo

- **La fórmula del peso** está en la vista: se cambia con una migración nueva
  (`CREATE OR REPLACE VIEW riesgo_historico ...`), nunca editando la `002` ni la `003`.
- **Cualquier cambio** en pesos, umbrales o parámetros sube la versión del modelo
  (`version_modelo` en la respuesta) y se anota en esta guía.

## Fuera de alcance por ahora

Lo muestra el Figma, pero no hay datos o no se ha definido:

- Factores de lluvia, pendiente del terreno y estado del drenaje; personas en la zona.
- Recalcular el modelo bajo demanda o con una frecuencia fija (hoy se calcula en cada
  consulta).
- Ajustar pesos desde la web y alertar a Protección Civil.
- Filtrar el mapa y el resumen por origen (solo reportes o solo datasets) o por
  dataset: la vista ya trae `origen` y `dataset_id`, solo falta el parámetro.
