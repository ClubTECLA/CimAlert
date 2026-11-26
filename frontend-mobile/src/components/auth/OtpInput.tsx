import { useRef } from 'react';
import { NativeSyntheticEvent, StyleSheet, TextInput, TextInputKeyPressEventData, View } from 'react-native';
import { colors } from '@/theme/colors';
import { fontFamily } from '@/theme/typography';
import { aplicarTexto, OTP_LENGTH } from './otp';
import { useAlEnfocar } from './teclado';

export { OTP_LENGTH };

type Props = {
  /** Arreglo de longitud fija (`OTP_LENGTH`), una posición por casilla — un string
   * simple no sirve porque al unir los dígitos se pierde la posición de las
   * casillas vacías en medio (ej. ['1','','3'] no es lo mismo que '13'). */
  value: string[];
  onChange: (value: string[]) => void;
};

export function OtpInput({ value, onChange }: Props) {
  const inputs = useRef<Array<TextInput | null>>([]);
  // Dónde está el cursor en cada casilla, según la propia casilla: aplicarTexto lo usa para saber de qué
  // lado quedó el dígito anterior. No se fija el cursor con `selection`: en iOS, cuando React Native lo
  // corrige a la fuerza no avisa, y la posición guardada quedaría distinta a la real.
  const cursores = useRef<number[]>([]);
  const alEnfocar = useAlEnfocar();
  const digits = Array.from({ length: OTP_LENGTH }, (_, i) => value[i] ?? '');

  const setDigit = (index: number, texto: string) => {
    const { digitos, foco } = aplicarTexto(digits, index, texto, cursores.current[index]);
    onChange(digitos);
    if (foco !== null) inputs.current[foco]?.focus();
  };

  const handleKeyPress = (index: number, e: NativeSyntheticEvent<TextInputKeyPressEventData>) => {
    if (e.nativeEvent.key === 'Backspace' && !digits[index] && index > 0) {
      inputs.current[index - 1]?.focus();
    }
  };

  return (
    <View style={styles.row}>
      {digits.map((digit, index) => (
        <TextInput
          key={index}
          ref={(ref) => {
            inputs.current[index] = ref;
          }}
          style={styles.box}
          value={digit}
          onChangeText={(text) => setDigit(index, text)}
          onKeyPress={(e) => handleKeyPress(index, e)}
          onFocus={alEnfocar}
          onSelectionChange={(e) => {
            cursores.current[index] = e.nativeEvent.selection.start;
          }}
          keyboardType="number-pad"
          // Sin maxLength: una casilla llena tiene que aceptar el dígito nuevo para reemplazarlo, y
          // pegar el código completo tiene que llegar entero (lo resuelve aplicarTexto).
          textContentType="oneTimeCode"
          autoComplete="sms-otp"
          textAlign="center"
        />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    justifyContent: 'center',
    gap: 8,
    width: '100%',
  },
  // Se reparten el ancho de la fila (hasta 74 px cada una) para que las 6 casillas quepan en pantallas angostas.
  box: {
    flex: 1,
    maxWidth: 74,
    aspectRatio: 74 / 84,
    borderRadius: 14,
    borderWidth: 1,
    borderColor: 'rgba(0,0,0,0.14)',
    backgroundColor: colors.white,
    fontFamily: fontFamily.semiBold,
    fontSize: 24,
    color: colors.textBody,
  },
});
