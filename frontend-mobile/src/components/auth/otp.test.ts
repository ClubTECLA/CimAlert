import { describe, expect, it } from 'vitest';
import { aplicarTexto, OTP_LENGTH } from './otp';

const vacio = () => Array<string>(OTP_LENGTH).fill('');

describe('aplicarTexto', () => {
  it('un dígito en una casilla vacía la llena y pasa el foco a la siguiente', () => {
    expect(aplicarTexto(vacio(), 0, '4')).toEqual({ digitos: ['4', '', '', '', '', ''], foco: 1 });
  });

  it('en la última casilla el foco se queda', () => {
    expect(aplicarTexto(vacio(), 5, '9').foco).toBeNull();
  });

  it('borrar deja la casilla vacía sin mover el foco', () => {
    expect(aplicarTexto(['1', '2', '', '', '', ''], 1, '')).toEqual({ digitos: ['1', '', '', '', '', ''], foco: null });
  });

  it('ignora lo que no es dígito', () => {
    expect(aplicarTexto(vacio(), 0, 'a').digitos[0]).toBe('');
  });

  it.each([
    ['cursor antes del dígito anterior', '40'],
    ['cursor después del dígito anterior', '04'],
  ])('escribir en una casilla llena reemplaza el dígito (%s)', (_, texto) => {
    const lleno = ['0', '0', '0', '0', '0', '0'];
    expect(aplicarTexto(lleno, 2, texto)).toEqual({ digitos: ['0', '0', '4', '0', '0', '0'], foco: 3 });
  });

  it('repetir el mismo dígito en una casilla llena la deja igual y avanza', () => {
    expect(aplicarTexto(['7', '', '', '', '', ''], 0, '77')).toEqual({ digitos: ['7', '', '', '', '', ''], foco: 1 });
  });

  it('pegar el código completo en cualquier casilla lo reparte en todas', () => {
    expect(aplicarTexto(vacio(), 3, '482234')).toEqual({ digitos: ['4', '8', '2', '2', '3', '4'], foco: 5 });
  });

  it('pegar el código en una casilla que ya tenía un dígito aplica el código pegado, no el anterior', () => {
    // El caso que encontró CodeRabbit: la casilla tiene "0" y se pega "482234" -> llega "0482234".
    const lleno = ['0', '0', '0', '0', '0', '0'];
    expect(aplicarTexto(lleno, 0, '0482234').digitos).toEqual(['4', '8', '2', '2', '3', '4']);
  });

  it('también si Android deja el cursor antes del dígito anterior y llega "4822340"', () => {
    const lleno = ['0', '0', '0', '0', '0', '0'];
    expect(aplicarTexto(lleno, 0, '4822340').digitos).toEqual(['4', '8', '2', '2', '3', '4']);
  });

  it('con el cursor antes del dígito, un código que empieza y termina con ese dígito se lee bien', () => {
    // El caso de la 2a revisión de CodeRabbit: casilla con "4", cursor en 0, se pega "482234" -> "4822344".
    // Sin la posición del cursor no se sabe si el "4" anterior es el primero o el último.
    const lleno = ['4', '', '', '', '', ''];
    expect(aplicarTexto(lleno, 0, '4822344', 0).digitos).toEqual(['4', '8', '2', '2', '3', '4']);
  });

  it('con el cursor después del dígito, el mismo texto se lee al revés', () => {
    const lleno = ['4', '', '', '', '', ''];
    expect(aplicarTexto(lleno, 0, '4482234', 1).digitos).toEqual(['4', '8', '2', '2', '3', '4']);
  });

  it('el código pegado puede empezar con el mismo dígito que ya estaba', () => {
    // Casilla con "4" y se pega "482234" (cursor después, como lo fija OtpInput) -> "4482234".
    const lleno = ['4', '', '', '', '', ''];
    expect(aplicarTexto(lleno, 0, '4482234').digitos).toEqual(['4', '8', '2', '2', '3', '4']);
  });

  it('un código completo que reemplaza el contenido de una casilla llena se reparte entero', () => {
    // El caso de la 3a revisión de CodeRabbit: la casilla tiene "4" y llega exactamente "482234".
    expect(aplicarTexto(['4', '', '', '', '', ''], 0, '482234', 1).digitos).toEqual(['4', '8', '2', '2', '3', '4']);
  });

  it('pegar un código con espacios o guiones toma solo los dígitos', () => {
    expect(aplicarTexto(vacio(), 0, '482 234').digitos).toEqual(['4', '8', '2', '2', '3', '4']);
  });
});
