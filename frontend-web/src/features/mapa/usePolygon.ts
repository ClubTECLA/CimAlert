import { useContext } from "react";
import { PolygonSelectionContext } from "./PolygonContext.ts";

export function usePolygon() {
    const context = useContext(PolygonSelectionContext);
    if (!context) {
        throw new Error("usePolygon must be used within a PolygonSelectionProvider");
    }
    return context;
}
