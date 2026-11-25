import PanelChrome from "./PanelChrome";
import { useCommonPanel } from "./useCommonPanel";

/**
 * Monta el panel abierto dentro del árbol de providers. Antes lo hacía CommonPanelProvider, pero eso dejaba las vistas
 * del panel fuera de los providers que se montan debajo de él (p. ej. el área dibujada) y sus hooks fallaban. Aquí se
 * lee la entrada del contexto y el chrome se monta donde main.tsx decide, con todos los providers ya disponibles.
 */
export default function PanelChromeMount() {
    const { currentPanel } = useCommonPanel();
    return currentPanel ? <PanelChrome entry={currentPanel} /> : null;
}
