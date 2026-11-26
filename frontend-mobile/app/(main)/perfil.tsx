import React from 'react';
import { router } from 'expo-router';
import { PerfilScreen } from '@/screens/PerfilScreen';
import { puedeSimularSesion, useEsInvitado, useSessionStore } from '@/lib/session';
import { sesionApi } from '@/features/sesion/api';
import { useMiPerfil, useMisReportes } from '@/features/perfil/hooks';
import { nombreVisible } from '@/features/perfil/nombre';

export default function Perfil() {
  // Al limpiar la sesión, app/_layout.tsx saca el mapa del historial y vuelve a la Bienvenida.
  const limpiarSesion = useSessionStore((s) => s.clear);
  const esInvitado = useEsInvitado();
  const perfil = useMiPerfil();
  // Misma consulta que "Mis reportes": comparten caché, así que no se pide dos veces.
  const misReportes = useMisReportes();

  const cerrarSesion = () => {
    // Avisa al backend sin esperar: la sesión se cierra en el teléfono aunque la petición falle.
    if (!esInvitado && !puedeSimularSesion) {
      sesionApi.logout().catch((error: unknown) => console.warn('Logout request failed:', error));
    }
    // app/_layout.tsx borra de la caché el perfil y los reportes del usuario al quedar sin token.
    limpiarSesion();
  };

  return (
    <PerfilScreen
      nombre={esInvitado ? 'Invitado' : perfil.data ? nombreVisible(perfil.data.username) : ''}
      correo={esInvitado ? undefined : perfil.data?.email}
      reportesActivos={!esInvitado && misReportes.isSuccess ? misReportes.activos.length : undefined}
      reportesCerrados={!esInvitado && misReportes.isSuccess ? misReportes.cerrados.length : undefined}
      cargando={!esInvitado && perfil.isPending && !perfil.isPaused}
      sinConexion={!esInvitado && perfil.isPending && perfil.isPaused}
      error={!esInvitado && perfil.isError}
      onReintentar={() => void perfil.refetch()}
      onBack={() => router.back()}
      onMisReportes={() => router.push('/(main)/mis-reportes')}
      onAlertas={() => router.push('/(main)/alertas')}
      onCerrarSesion={cerrarSesion}
    />
  );
}