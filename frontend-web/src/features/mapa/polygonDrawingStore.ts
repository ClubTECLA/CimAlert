// Bandera global de "se está dibujando un polígono". LocationPickerProvider está por encima de
// PolygonSelectionProvider en el árbol de providers, así que no puede leer su contexto: esta señal externa permite
// que su handler global de clic ignore los clics del dibujo sin invertir el orden de providers ni recrear el marcador.

let drawing = false;
const listeners = new Set<() => void>();

export const polygonDrawingStore = {
    get: () => drawing,
    set(next: boolean) {
        if (drawing === next) return;
        drawing = next;
        for (const listener of listeners) listener();
    },
    subscribe(listener: () => void) {
        listeners.add(listener);
        return () => { listeners.delete(listener); };
    },
};
