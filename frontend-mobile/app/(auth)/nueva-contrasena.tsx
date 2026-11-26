import { useState } from 'react';
import { Alert, StyleSheet, Text } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { AuthScreenShell } from '@/components/auth/AuthScreenShell';
import { PasswordField } from '@/components/auth/PasswordField';
import { PrimaryButton } from '@/components/auth/PrimaryButton';
import { StepDots } from '@/components/auth/StepDots';
import { esErrorDeCodigo, mensajeErrorAuth, sesionApi } from '@/features/sesion/api';
import { PASSWORD_MIN } from '@/lib/correo';
import { puedeSimularSesion } from '@/lib/session';
import { useVigenciaDePantalla } from '@/hooks/useVigenciaDePantalla';
import { colors } from '@/theme/colors';
import { fontFamily } from '@/theme/typography';

/** Paso 3 de recuperar contraseña: cambia la contraseña con el token que devolvió la verificación del paso 2. */
export default function NuevaContrasenaScreen() {
  const router = useRouter();
  const { email = '', resetToken = '' } = useLocalSearchParams<{ email?: string; resetToken?: string }>();
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [enviando, setEnviando] = useState(false);
  // Si se sale de la pantalla mientras se cambia la contraseña, su respuesta ya no muestra alertas: el
  // "Pedir código nuevo" de un error tardío llevaría a otra pantalla.
  const vigencia = useVigenciaDePantalla(() => setEnviando(false));

  const canSubmit = password.length >= PASSWORD_MIN && password === confirmPassword && !enviando;

  // El backend no devuelve token: se vuelve al inicio de sesión con el correo ya escrito.
  const irAlLogin = () => router.dismissTo({ pathname: '/(auth)/auth', params: { tab: 'login', email } });

  const confirmar = async () => {
    setEnviando(true);
    const marca = vigencia.iniciar();
    try {
      // Con mocks no hay backend: se recorre el diseño sin cambiar nada.
      if (!puedeSimularSesion) await sesionApi.restablecerContrasena(resetToken, password);
      if (!vigencia.vigente(marca)) return;
      Alert.alert('Contraseña actualizada', 'Ya puedes iniciar sesión con tu nueva contraseña.', [
        { text: 'Iniciar sesión', onPress: irAlLogin },
      ]);
    } catch (error) {
      if (!vigencia.vigente(marca)) return;
      // El token vence a los 15 minutos de verificar el código, o dejó de valer porque se pidió otro código.
      // El código ya se usó, así que no sirve volver al paso 2: se vuelve al 1 (con el correo escrito).
      if (esErrorDeCodigo(error)) {
        Alert.alert(
          'No se pudo cambiar la contraseña',
          'Se acabó el tiempo para cambiar la contraseña. Pide un código nuevo.',
          [
            { text: 'Cancelar', style: 'cancel' },
            { text: 'Pedir código nuevo', onPress: () => router.dismissTo('/(auth)/recuperar') },
          ]
        );
        return;
      }
      Alert.alert('No se pudo cambiar la contraseña', mensajeErrorAuth(error, 'recuperacion'));
    } finally {
      if (vigencia.vigente(marca)) setEnviando(false);
    }
  };

  return (
    <AuthScreenShell title="Recuperar contraseña">
      <PasswordField label="Nueva contraseña" value={password} onChangeText={setPassword} />
      {password.length > 0 && password.length < PASSWORD_MIN && (
        <Text style={styles.ayuda}>Usa al menos {PASSWORD_MIN} caracteres.</Text>
      )}
      <PasswordField
        label="Confirmar contraseña"
        value={confirmPassword}
        onChangeText={setConfirmPassword}
      />
      {confirmPassword.length > 0 && password !== confirmPassword && (
        <Text style={styles.ayuda}>Las contraseñas no coinciden.</Text>
      )}

      <PrimaryButton
        label={enviando ? 'Guardando…' : 'Confirmar cambios'}
        disabled={!canSubmit}
        onPress={() => void confirmar()}
      />

      <StepDots total={3} current={3} />
    </AuthScreenShell>
  );
}

const styles = StyleSheet.create({
  ayuda: {
    marginTop: -8,
    fontFamily: fontFamily.regular,
    fontSize: 12,
    color: colors.textMuted,
  },
});
