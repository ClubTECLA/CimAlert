import { apiRequest } from "@/lib/http";
import type {
    ApiRecoveryRequest,
    ApiResetPasswordRequest,
    ApiVerifyRecoveryRequest,
    ApiVerifyRecoveryResponse,
} from "@/types/api-types";

/** Solicita que el backend envíe un PIN de recuperación al correo, si corresponde. */
export function solicitarRecuperacion(request: ApiRecoveryRequest): Promise<void> {
    return apiRequest<void>("/auth/recovery", {
        method: "POST",
        json: request,
        auth: false,
    });
}

/** Verifica el PIN y devuelve el token de un solo uso para establecer la contraseña. */
export function verificarRecuperacion(request: ApiVerifyRecoveryRequest): Promise<ApiVerifyRecoveryResponse> {
    return apiRequest<ApiVerifyRecoveryResponse>("/auth/recovery/verify", {
        method: "POST",
        json: request,
        auth: false,
    });
}

/** Guarda la nueva contraseña con el token de la verificación; el backend responde 204. */
export function restablecerContrasena(request: ApiResetPasswordRequest): Promise<void> {
    return apiRequest<void>("/auth/recovery/reset", {
        method: "POST",
        json: request,
        auth: false,
    });
}
