import { useState } from 'react';
import { Alert } from 'react-native';
import { useRouter } from 'expo-router';
import { AuthScreenShell } from '@/components/auth/AuthScreenShell';
import { TextField } from '@/components/auth/TextField';
import { PrimaryButton } from '@/components/auth/PrimaryButton';
import { StepDots } from '@/components/auth/StepDots';
import { mensajeErrorAuth, sesionApi } from '@/features/sesion/api';
import { esCorreoValido } from '@/lib/correo';
import { puedeSimularSesion } from '@/lib/session';

/** Paso 1 de recuperar contraseña: pide el código que llega al correo. */
export default function RecuperarScreen() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [enviando, setEnviando] = useState(false);

  const enviarCodigo = async () => {
    const correo = email.trim();
    setEnviando(true);
    try {
      // Con mocks no hay backend: se recorre el diseño sin mandar correo.
      // El código vence a los 15 min de que el backend lo crea: se toma la hora antes de pedirlo, para
      // que una respuesta lenta no haga que el contador del paso 2 muestre tiempo que ya no existe.
      const pedidoEn = Date.now();
      if (!puedeSimularSesion) await sesionApi.solicitarRecuperacion(correo);
      router.push({ pathname: '/(auth)/verificar', params: { email: correo, pedidoEn: String(pedidoEn) } });
    } catch (error) {
      Alert.alert('No se pudo enviar el código', mensajeErrorAuth(error, 'recuperacion'));
    } finally {
      setEnviando(false);
    }
  };

  return (
    <AuthScreenShell title="Recuperar contraseña">
      <TextField
        label="Correo electrónico"
        placeholder="tucorreo@ejemplo.com"
        value={email}
        onChangeText={setEmail}
        keyboardType="email-address"
        autoCapitalize="none"
        autoCorrect={false}
        autoComplete="email"
        textContentType="emailAddress"
      />

      <PrimaryButton
        label={enviando ? 'Enviando…' : 'Enviar código'}
        disabled={!esCorreoValido(email) || enviando}
        onPress={() => void enviarCodigo()}
      />

      <StepDots total={3} current={1} />
    </AuthScreenShell>
  );
}
