import { useEffect, useState } from 'react';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { Alert, Pressable, StyleSheet, Text, View } from 'react-native';
import { AuthScreenShell } from '@/components/auth/AuthScreenShell';
import { OTP_LENGTH, OtpInput } from '@/components/auth/OtpInput';
import { PrimaryButton } from '@/components/auth/PrimaryButton';
import { StepDots } from '@/components/auth/StepDots';
import { mensajeErrorAuth, RECUPERACION, sesionApi } from '@/features/sesion/api';
import { puedeSimularSesion } from '@/lib/session';
import { useVigenciaDePantalla } from '@/hooks/useVigenciaDePantalla';
import { colors } from '@/theme/colors';
import { fontFamily } from '@/theme/typography';

const OTP_SECONDS = RECUPERACION.minutosVigencia * 60;

function formatTime(totalSeconds: number) {
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
}

const codigoVacio = () => Array<string>(OTP_LENGTH).fill('');

/** Paso 2 de recuperar contraseña: el código que se envía al correo. El backend no tiene una
 * llamada solo para validarlo: se manda junto con la nueva contraseña en el paso 3. */
export default function VerificarScreen() {
  const router = useRouter();
  const { email = '', pedidoEn } = useLocalSearchParams<{ email?: string; pedidoEn?: string }>();

  const [code, setCode] = useState<string[]>(codigoVacio);
  // Se cuenta desde que se pidió el código (antes de la petición, en el paso 1 o en "Reenviar código"),
  // no desde que se abrió la pantalla ni desde que respondió el backend.
  const [enviadoEn, setEnviadoEn] = useState(() => Number(pedidoEn) || Date.now());
  const [secondsLeft, setSecondsLeft] = useState(OTP_SECONDS);
  const [reenviando, setReenviando] = useState(false);
  // Si se sale de la pantalla mientras se reenvía, la respuesta ya no cambia nada ni muestra alertas.
  const vigencia = useVigenciaDePantalla(() => setReenviando(false));

  useEffect(() => {
    const tick = () =>
      setSecondsLeft(Math.max(0, OTP_SECONDS - Math.floor((Date.now() - enviadoEn) / 1000)));
    tick();
    const interval = setInterval(tick, 1000);
    return () => clearInterval(interval);
  }, [enviadoEn]);

  const vencido = secondsLeft === 0;
  // Mientras se reenvía, el código escrito está por quedar invalidado: no se deja pasar con él.
  const canVerify = !vencido && !reenviando && code.every((digit) => /^[0-9]$/.test(digit));

  const handleVerify = () =>
    router.push({ pathname: '/(auth)/nueva-contrasena', params: { email, pin: code.join('') } });

  const reenviar = async () => {
    setReenviando(true);
    const pedidoAhora = Date.now();
    const marca = vigencia.iniciar();
    try {
      if (!puedeSimularSesion) await sesionApi.solicitarRecuperacion(email);
      if (!vigencia.vigente(marca)) return;
      // El backend cancela el código anterior al mandar uno nuevo.
      setCode(codigoVacio());
      setEnviadoEn(pedidoAhora);
      Alert.alert('Código reenviado', `Te mandamos un código nuevo a ${email}.`);
    } catch (error) {
      if (vigencia.vigente(marca)) Alert.alert('No se pudo reenviar el código', mensajeErrorAuth(error, 'recuperacion'));
    } finally {
      if (vigencia.vigente(marca)) setReenviando(false);
    }
  };

  return (
    <AuthScreenShell title="Ingresa el código">
      <StepDots total={3} current={2} />

      <Text style={styles.subtitle}>Escribe el código de {OTP_LENGTH} dígitos</Text>

      <Text style={styles.sentTo}>
        <Text style={styles.sentToMuted}>Código enviado a </Text>
        {email || 'tu correo'}
      </Text>

      <OtpInput value={code} onChange={setCode} />

      <View style={styles.metaRow}>
        <Text style={styles.expiresLabel}>
          {vencido ? (
            <Text style={styles.expiredLabel}>El código venció</Text>
          ) : (
            <>
              <Text style={styles.sentToMuted}>Vence en: </Text>
              {formatTime(secondsLeft)}
            </>
          )}
        </Text>
        <Pressable onPress={() => void reenviar()} disabled={reenviando} hitSlop={8}>
          <Text style={[styles.resendLabel, reenviando && styles.resendDisabled]}>
            {reenviando ? 'Reenviando…' : 'Reenviar código'}
          </Text>
        </Pressable>
      </View>

      <Text style={styles.attemptsLabel}>
        Tienes {RECUPERACION.intentosPin} intentos por código y puedes pedir hasta {RECUPERACION.codigosPorHora}{' '}
        códigos por hora
      </Text>

      <PrimaryButton label="Verificar" disabled={!canVerify} onPress={handleVerify} />
    </AuthScreenShell>
  );
}

const styles = StyleSheet.create({
  subtitle: {
    fontFamily: fontFamily.semiBold,
    fontSize: 19,
    color: colors.black,
    marginTop: -4,
  },
  sentTo: {
    fontFamily: fontFamily.semiBold,
    fontSize: 15,
    color: colors.black,
  },
  sentToMuted: {
    color: colors.textMutedSoft,
  },
  metaRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  expiresLabel: {
    fontFamily: fontFamily.semiBold,
    fontSize: 13,
    color: colors.black,
  },
  expiredLabel: {
    color: colors.danger,
  },
  resendLabel: {
    fontFamily: fontFamily.semiBold,
    fontSize: 13,
    color: colors.primaryLight,
  },
  resendDisabled: {
    opacity: 0.5,
  },
  attemptsLabel: {
    fontFamily: fontFamily.semiBold,
    fontSize: 11,
    color: colors.textMutedSoft,
    textAlign: 'center',
  },
});
