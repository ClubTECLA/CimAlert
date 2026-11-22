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
import { colors } from '@/theme/colors';
import { fontFamily } from '@/theme/typography';

/** Paso 3 de recuperar contraseña: aquí se manda el código del paso 2 junto con la nueva contraseña. */
export default function NuevaContrasenaScreen() {
  const router = useRouter();
  const { email = '', pin = '' } = useLocalSearchParams<{ email?: string; pin?: string }>();
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [enviando, setEnviando] = useState(false);

  const canSubmit = password.length >= PASSWORD_MIN && password === confirmPassword && !enviando;

  // El backend no devuelve token: se vuelve al inicio de sesión con el correo ya escrito.
  const irAlLogin = () => router.dismissTo({ pathname: '/(auth)/auth', params: { tab: 'login', email } });

  const confirmar = async () => {
    setEnviando(true);
    try {
      // Con mocks no hay backend: se recorre el diseño sin cambiar nada.
      if (!puedeSimularSesion) await sesionApi.verificarRecuperacion(email, pin, password);
      Alert.alert('Contraseña actualizada', 'Ya puedes iniciar sesión con tu nueva contraseña.', [
        { text: 'Iniciar sesión', onPress: irAlLogin },
      ]);
    } catch (error) {
      const mensaje = mensajeErrorAuth(error, 'recuperacion');
      Alert.alert(
        'No se pudo cambiar la contraseña',
        mensaje,
        // Si el código está mal, se vuelve al paso 2 (que conserva lo que se escribió ahí) para corregirlo.
        esErrorDeCodigo(error)
          ? [
              { text: 'Cancelar', style: 'cancel' },
              { text: 'Revisar código', onPress: () => router.back() },
            ]
          : undefined
      );
    } finally {
      setEnviando(false);
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
