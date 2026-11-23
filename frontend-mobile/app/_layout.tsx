import { useEffect } from 'react';
import { Alert } from 'react-native';
import { Stack } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import * as WebBrowser from 'expo-web-browser';
import { StatusBar } from 'expo-status-bar';
import { QueryClientProvider } from '@tanstack/react-query';
import { queryClient } from '@/lib/query-client';
import { onSesionExpirada, useSesionCargada, useSessionStore } from '@/lib/session';
import { perfilKeys } from '@/features/perfil/hooks';
import { iniciarSesionDev } from '@/lib/devAuth';
// Define la tarea de zonas de peligro en el ámbito global (debe existir aunque no haya vistas).
import '@/lib/alertasCercanas';
import {
  useFonts,
  Inter_400Regular,
  Inter_500Medium,
  Inter_600SemiBold,
  Inter_700Bold,
} from '@expo-google-fonts/inter';

SplashScreen.preventAutoHideAsync().catch(() => {});
// Requerido por expo-auth-session para cerrar el navegador del login
// (Google, etc.) y devolver el control a la app.
WebBrowser.maybeCompleteAuthSession();

export default function RootLayout() {
  const [fontsLoaded, fontError] = useFonts({
    Inter_400Regular,
    Inter_500Medium,
    Inter_600SemiBold,
    Inter_700Bold,
  });
  const fontsReady = fontsLoaded || !!fontError;
  // La sesión guardada decide si se abre la Bienvenida o el mapa: hasta leerla sigue la splash.
  const sesionCargada = useSesionCargada();
  const modo = useSessionStore((s) => s.modo);
  const listo = fontsReady && sesionCargada;

  // Solo desarrollo con backend real (ver lib/devAuth): se espera a leer la sesión guardada para no pisarla.
  useEffect(() => {
    if (sesionCargada) void iniciarSesionDev();
  }, [sesionCargada]);

  // Al cambiar de sesión (login, registro, cerrar sesión o sesión vencida) lo que se pidió con el
  // token anterior, o sin token como invitado, ya no vale: se vuelve a pedir con el nuevo. Sin esto,
  // al entrar desde el modo invitado el mapa seguía con el error de los reportes que pidió sin token.
  useEffect(
    () =>
      useSessionStore.subscribe((estado, anterior) => {
        if (estado.token === anterior.token) return;
        // Los datos de la cuenta (perfil, mis reportes) se borran en cualquier cambio de token: al cerrar
        // sesión y también al pasar de una cuenta a otra, para que la nueva no vea los del anterior.
        queryClient.removeQueries({ queryKey: perfilKeys.me });
        queryClient.removeQueries({ queryKey: perfilKeys.misReportes });
        // Una petición que seguía en curso salió con el token anterior (o sin token, como invitado), y
        // `invalidateQueries` no cancela la primera carga de una consulta: se cancela antes de volver a pedir.
        void queryClient.cancelQueries().then(() => queryClient.invalidateQueries());
      }),
    [],
  );

  // El backend rechazó la sesión (http.ts ya la cerró y la navegación vuelve sola a la Bienvenida).
  useEffect(
    () =>
      onSesionExpirada(() =>
        Alert.alert('Tu sesión expiró', 'Vuelve a iniciar sesión para seguir reportando y comentando.')
      ),
    [],
  );

  useEffect(() => {
    if (listo) {
      SplashScreen.hideAsync().catch(() => {});
    }
  }, [listo]);

  // Si las fuentes fallan en cargar, seguimos con las del sistema en vez de
  // dejar la app trabada en la splash screen para siempre.
  if (!listo) {
    return null;
  }

  return (
    <QueryClientProvider client={queryClient}>
      {/* Sin sesión solo existe la Bienvenida; al elegir invitado o iniciar sesión se pasa al
          mapa, y al cerrar sesión expo-router saca el mapa del historial y vuelve a la
          Bienvenida. Login, registro y permisos quedan siempre disponibles. */}
      <Stack screenOptions={{ headerShown: false }}>
        <Stack.Protected guard={modo === null}>
          <Stack.Screen name="index" />
        </Stack.Protected>
        <Stack.Protected guard={modo !== null}>
          <Stack.Screen name="(main)" />
        </Stack.Protected>
      </Stack>
      <StatusBar style="dark" />
    </QueryClientProvider>
  );
}
