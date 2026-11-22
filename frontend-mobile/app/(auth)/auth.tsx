import { useEffect, useState } from 'react';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { Alert, Pressable, StyleSheet, Text, View } from 'react-native';
import { AuthScreenShell } from '@/components/auth/AuthScreenShell';
import { AuthTabs, AuthTab } from '@/components/auth/AuthTabs';
import { TextField } from '@/components/auth/TextField';
import { PasswordField } from '@/components/auth/PasswordField';
import { PrimaryButton } from '@/components/auth/PrimaryButton';
import { GoogleButton } from '@/components/auth/GoogleButton';
import { TermsCheckbox } from '@/components/auth/TermsCheckbox';
import { useGoogleSignIn } from '@/hooks/useGoogleSignIn';
import { iniciarSesionYEntrar } from '@/features/sesion/entrar';
import { mensajeErrorAuth, sesionApi } from '@/features/sesion/api';
import { esCorreoValido, PASSWORD_MIN } from '@/lib/correo';
import { puedeSimularSesion } from '@/lib/session';
import { colors } from '@/theme/colors';
import { fontFamily } from '@/theme/typography';

export default function AuthScreen() {
  const router = useRouter();
  const params = useLocalSearchParams<{ tab?: string; email?: string }>();
  const [tab, setTab] = useState<AuthTab>(params.tab === 'login' ? 'login' : 'signup');

  // Al volver de recuperar la contraseña llegan `tab=login` y el correo: la pantalla ya existía,
  // así que el estado inicial no basta.
  useEffect(() => {
    if (params.tab === 'login') setTab('login');
    if (params.email) setEmail(params.email);
  }, [params.tab, params.email]);

  const { signIn: signInWithGoogle } = useGoogleSignIn({
    onSuccess: (profile) => {
      Alert.alert(
        'Sesión iniciada con Google',
        `Bienvenido${profile.name ? `, ${profile.name}` : ''}${profile.email ? `\n${profile.email}` : ''}`,
        [{ text: 'OK', onPress: () => iniciarSesionYEntrar() }]
      );
    },
  });

  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [acceptedTerms, setAcceptedTerms] = useState(false);
  const [rememberMe, setRememberMe] = useState(false);

  const [enviando, setEnviando] = useState(false);

  const correoValido = esCorreoValido(email);
  const canSubmitSignup =
    !!username.trim() &&
    correoValido &&
    password.length >= PASSWORD_MIN &&
    password === confirmPassword &&
    acceptedTerms;
  const canSubmitLogin = correoValido && !!password;

  /** Pide el token al backend y entra al mapa; si falla, avisa y deja el formulario como estaba. */
  const autenticar = async (accion: 'login' | 'registro') => {
    setEnviando(true);
    try {
      const { access_token } =
        accion === 'login'
          ? await sesionApi.login(email.trim(), password)
          : await sesionApi.register(email.trim(), username.trim(), password);
      iniciarSesionYEntrar(access_token);
    } catch (error) {
      Alert.alert(
        accion === 'login' ? 'No se pudo iniciar sesión' : 'No se pudo crear la cuenta',
        mensajeErrorAuth(error, accion)
      );
    } finally {
      setEnviando(false);
    }
  };

  // Con mocks se entra con una sesión simulada para poder probar la app como usuario. El backend
  // crea la cuenta sin verificar el correo, así que el registro también entra directo.
  const registrarse = () => (puedeSimularSesion ? iniciarSesionYEntrar() : void autenticar('registro'));
  const iniciarSesion = () => (puedeSimularSesion ? iniciarSesionYEntrar() : void autenticar('login'));

  return (
    <AuthScreenShell title={tab === 'signup' ? 'Crear cuenta' : 'Iniciar sesión'}>
      <AuthTabs value={tab} onChange={setTab} />

      {tab === 'signup' ? (
        <>
          <TextField
            label="Nombre de usuario"
            placeholder="Alex Ramírez"
            value={username}
            onChangeText={setUsername}
            autoCapitalize="words"
            autoComplete="username"
            maxLength={100}
          />
          <CampoCorreo value={email} onChangeText={setEmail} />
          <PasswordField label="Contraseña" value={password} onChangeText={setPassword} />
          {password.length > 0 && password.length < PASSWORD_MIN && (
            <Text style={styles.ayuda}>Usa al menos {PASSWORD_MIN} caracteres.</Text>
          )}
          <PasswordField
            label="Confirmar contraseña"
            value={confirmPassword}
            onChangeText={setConfirmPassword}
          />
          <TermsCheckbox checked={acceptedTerms} onChange={setAcceptedTerms} />

          <PrimaryButton
            label={enviando ? 'Creando cuenta…' : 'Registrarse'}
            disabled={!canSubmitSignup || enviando}
            onPress={registrarse}
          />
        </>
      ) : (
        <>
          <CampoCorreo value={email} onChangeText={setEmail} />
          <PasswordField label="Contraseña" value={password} onChangeText={setPassword} />

          <View style={styles.loginLinksRow}>
            <Pressable style={styles.rememberRow} onPress={() => setRememberMe((v) => !v)}>
              <View style={[styles.checkbox, rememberMe && styles.checkboxChecked]} />
              <Text style={styles.rememberLabel}>Recordarme</Text>
            </Pressable>
            <Pressable onPress={() => router.push('/(auth)/recuperar')}>
              <Text style={styles.forgotLabel}>¿Olvidaste la contraseña?</Text>
            </Pressable>
          </View>

          <PrimaryButton
            label={enviando ? 'Ingresando…' : 'Iniciar sesión'}
            showArrow={false}
            disabled={!canSubmitLogin || enviando}
            onPress={iniciarSesion}
          />
        </>
      )}

      <View style={styles.divider}>
        <View style={styles.dividerLine} />
        <Text style={styles.dividerLabel}>O</Text>
        <View style={styles.dividerLine} />
      </View>

      <GoogleButton onPress={signInWithGoogle} />
    </AuthScreenShell>
  );
}

/** Campo de correo con el teclado y el autocompletado del sistema para correos. */
function CampoCorreo(props: { value: string; onChangeText: (texto: string) => void }) {
  return (
    <TextField
      label="Correo electrónico"
      placeholder="tucorreo@ejemplo.com"
      keyboardType="email-address"
      autoCapitalize="none"
      autoCorrect={false}
      autoComplete="email"
      textContentType="emailAddress"
      {...props}
    />
  );
}

const styles = StyleSheet.create({
  ayuda: {
    marginTop: -8,
    fontFamily: fontFamily.regular,
    fontSize: 12,
    color: colors.textMuted,
  },
  loginLinksRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  rememberRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  checkbox: {
    width: 20,
    height: 20,
    borderRadius: 6,
    borderWidth: 1.5,
    borderColor: colors.checkboxBorder,
  },
  checkboxChecked: {
    backgroundColor: colors.primary,
    borderColor: colors.primary,
  },
  rememberLabel: {
    fontFamily: fontFamily.medium,
    fontSize: 13,
    color: colors.textMuted,
  },
  forgotLabel: {
    fontFamily: fontFamily.semiBold,
    fontSize: 13,
    color: colors.link,
  },
  divider: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 14,
  },
  dividerLine: {
    flex: 1,
    height: 1,
    backgroundColor: colors.inputBorder,
  },
  dividerLabel: {
    fontFamily: fontFamily.regular,
    fontSize: 13,
    color: colors.textMuted,
  },
});
