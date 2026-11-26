/**
 * Liga una petición a la "visita" a una pantalla: al salir de la pantalla, las peticiones que sigan en
 * curso dejan de estar vigentes y su respuesta (buena o mala) se ignora, para que no navegue ni muestre
 * alertas sobre otra pantalla. Si se vuelve a la pantalla, las respuestas viejas siguen sin valer.
 */
export function crearVigencia() {
  let visita = 0;
  return {
    /** Al empezar una petición: devuelve la marca que luego se revisa con `vigente`. */
    iniciar: (): number => visita,
    /** true si desde que empezó la petición no se ha salido de la pantalla. */
    vigente: (marca: number): boolean => marca === visita,
    /** Al salir de la pantalla. */
    invalidar: (): void => {
      visita += 1;
    },
  };
}

export type Vigencia = ReturnType<typeof crearVigencia>;
