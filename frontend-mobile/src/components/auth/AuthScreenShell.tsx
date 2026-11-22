import { ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Keyboard,
  KeyboardAvoidingView,
  Platform,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  useWindowDimensions,
  View,
  Pressable,
} from 'react-native';
import { SafeAreaView, useSafeAreaInsets } from 'react-native-safe-area-context';
import { useRouter } from 'expo-router';
import { colors } from '@/theme/colors';
import { fontFamily } from '@/theme/typography';
import { AuthMapBackground } from './AuthMapBackground';
import { ArrowLeftIcon } from '../icons/ArrowLeftIcon';
import { desplazamientoParaVer, TecladoContext } from './teclado';

type Props = {
  title: string;
  children: ReactNode;
  showBackButton?: boolean;
};

/** Layout compartido por las pantallas de auth: fondo de mapa + título + contenido scrolleable. */
export function AuthScreenShell({ title, children, showBackButton = true }: Props) {
  const { height: windowHeight } = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const router = useRouter();

  // Al tocar un campo, la pantalla se recorre hasta dejarlo justo arriba del teclado. En Android, con
  // edge-to-edge (obligatorio desde Android 15) la ventana ya no se encoge con el teclado, así que
  // KeyboardAvoidingView no basta: se mide el campo y se mueve el scroll a mano.
  const scrollRef = useRef<ScrollView>(null);
  const scrollY = useRef(0);
  const tecladoArriba = useRef<number | null>(null);
  const [alturaTeclado, setAlturaTeclado] = useState(0);

  const mostrarCampoEnfocado = useCallback(() => {
    // Espera un momento: el teclado puede seguir abriéndose y el espacio de abajo, acomodándose.
    setTimeout(() => {
      const campo = TextInput.State.currentlyFocusedInput();
      const limite = tecladoArriba.current;
      if (!campo || limite === null) return;
      campo.measureInWindow((_x, y, _ancho, alto) => {
        const extra = desplazamientoParaVer(y + alto, limite);
        if (extra > 0) scrollRef.current?.scrollTo({ y: scrollY.current + extra, animated: true });
      });
    }, 80);
  }, []);

  useEffect(() => {
    const alMostrar = Keyboard.addListener('keyboardDidShow', (e) => {
      tecladoArriba.current = e.endCoordinates.screenY;
      setAlturaTeclado(e.endCoordinates.height);
      mostrarCampoEnfocado();
    });
    const alOcultar = Keyboard.addListener('keyboardDidHide', () => {
      tecladoArriba.current = null;
      setAlturaTeclado(0);
    });
    return () => {
      alMostrar.remove();
      alOcultar.remove();
    };
  }, [mostrarCampoEnfocado]);

  const teclado = useMemo(() => ({ mostrarCampoEnfocado }), [mostrarCampoEnfocado]);

  return (
    <TecladoContext.Provider value={teclado}>
      <View style={styles.root}>
        {/* Sin "top": el mapa debe llegar hasta el borde superior de la
            pantalla, igual que en el diseño (va detrás de la barra de estado). */}
        <SafeAreaView style={styles.safeArea} edges={['bottom']}>
          <KeyboardAvoidingView
            style={styles.flex}
            behavior={Platform.OS === 'ios' ? 'padding' : undefined}
          >
            <ScrollView
              ref={scrollRef}
              style={styles.flex}
              // Con el teclado abierto, espacio extra abajo para poder subir hasta el último campo.
              contentContainerStyle={[
                styles.scrollContent,
                Platform.OS === 'android' && { paddingBottom: styles.scrollContent.paddingBottom + alturaTeclado },
              ]}
              onScroll={(e) => {
                scrollY.current = e.nativeEvent.contentOffset.y;
              }}
              scrollEventThrottle={16}
              keyboardShouldPersistTaps="handled"
              showsVerticalScrollIndicator={false}
              bounces
            >
              {/* El mapa va dentro del scroll (no fijo) para que se desplace
                  junto con el formulario y nunca quede el texto montado sobre
                  los pines del mapa. */}
              <AuthMapBackground height={windowHeight * 0.52} />
              <View style={styles.content}>
                <Text style={styles.title}>{title}</Text>
                <View style={styles.body}>{children}</View>
              </View>
            </ScrollView>
          </KeyboardAvoidingView>
        </SafeAreaView>

        {/* Botón de retroceso superpuesto */}
        {showBackButton && (
          <Pressable
            style={[styles.backButton, { top: Math.max(insets.top + 16, 16) }]}
            onPress={() => {
              if (router.canGoBack()) {
                router.back();
              } else {
                router.replace('/');
              }
            }}
          >
            <ArrowLeftIcon size={24} color={colors.textMuted} />
          </Pressable>
        )}
      </View>
    </TecladoContext.Provider>
  );
}

const styles = StyleSheet.create({
  root: {
    flex: 1,
    backgroundColor: colors.white,
  },
  flex: {
    flex: 1,
  },
  safeArea: {
    flex: 1,
  },
  scrollContent: {
    flexGrow: 1,
    paddingBottom: 32,
  },
  content: {
    paddingHorizontal: 24,
    backgroundColor: colors.white,
  },
  title: {
    fontFamily: fontFamily.bold,
    fontSize: 30,
    color: colors.black,
    marginBottom: 20,
  },
  body: {
    gap: 18,
  },
  backButton: {
    position: 'absolute',
    left: 20,
    width: 44,
    height: 44,
    borderRadius: 22,
    backgroundColor: colors.white,
    justifyContent: 'center',
    alignItems: 'center',
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 3,
  },
});
