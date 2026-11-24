import React from 'react';
import { Alert } from 'react-native';
import { router } from 'expo-router';
import { guardarRadioAlertaKm } from '@/lib/alertasCercanas';
import { useFiltrosStore } from '@/features/mapa/filtros/filtrosStore';
import { AlertasScreen } from '@/screens/AlertasScreen';

export default function Alertas() {
  return (
    <AlertasScreen
      onBack={() => router.back()}
      onGuardar={({ radioKm, categorias }) => {
        // Se espera solo la escritura (milisegundos) para que Perfil ya lea el radio nuevo al volver.
        // Si falla, la pantalla sigue abierta para que el usuario lo intente de nuevo.
        guardarRadioAlertaKm(radioKm)
          .then(() => {
            // Las categorías se aplican al filtro del mapa conservando el periodo y las capas actuales.
            const { filtros, aplicar } = useFiltrosStore.getState();
            aplicar({ ...filtros, categorias });
            router.back();
          })
          .catch((err) => {
            console.warn('[alertas] no se pudo guardar el radio:', err);
            Alert.alert('No se pudo guardar', 'Intenta de nuevo en un momento.');
          });
      }}
    />
  );
}