import { describe, expect, it } from "vitest";
import { isVideoFile, prepararVideo } from "./prepararVideo";

// En Node no hay reproductor: solo se prueban los rechazos previos a abrir el video. La duración se probó en el navegador.
describe("prepararVideo", () => {
    it("distingue los videos por tipo o, sin tipo, por extensión", () => {
        expect(isVideoFile(new File(["x"], "a.mp4", { type: "video/mp4" }))).toBe(true);
        expect(isVideoFile(new File(["x"], "IMG_0001.MOV", { type: "" }))).toBe(true);
        expect(isVideoFile(new File(["x"], "foto.jpg", { type: "image/jpeg" }))).toBe(false);
    });

    it("rechaza los .mov y cualquier video que no sea MP4", async () => {
        await expect(prepararVideo(new File(["x"], "IMG_0001.MOV", { type: "video/quicktime" })))
            .rejects.toThrow("Formato de video no soportado. Usa un MP4");
    });

    it("rechaza videos de más de 8 MB sin abrirlos", async () => {
        const big = new File([new Uint8Array(8 * 1024 * 1024 + 1)], "largo.mp4", { type: "video/mp4" });
        await expect(prepararVideo(big)).rejects.toThrow("El video pesa más de 8 MB.");
    });
});
