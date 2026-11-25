package usuario

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SMTPConfig agrupa los datos de conexión. Todo viene de variables de entorno.
type SMTPConfig struct {
	Host string // p. ej. smtp-relay.brevo.com
	Port string // 587 (STARTTLS)
	User string // login SMTP de Brevo (no siempre es el correo de la cuenta)
	Pass string // clave SMTP generada en Brevo
	From string // remitente verificado en Brevo
	Name string // nombre visible del remitente
}

// SMTPConfigFromEnv lee SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, MAIL_FROM
// y MAIL_FROM_NAME (opcional, default "CimAlert").
func SMTPConfigFromEnv() (SMTPConfig, error) {
	c := SMTPConfig{
		Host: os.Getenv("SMTP_HOST"),
		Port: os.Getenv("SMTP_PORT"),
		User: os.Getenv("SMTP_USER"),
		Pass: os.Getenv("SMTP_PASS"),
		From: os.Getenv("MAIL_FROM"),
		Name: os.Getenv("MAIL_FROM_NAME"),
	}
	if c.Port == "" {
		c.Port = "587"
	}
	if c.Name == "" {
		c.Name = "CimAlert"
	}
	if c.Host == "" || c.User == "" || c.Pass == "" || c.From == "" {
		return SMTPConfig{}, fmt.Errorf("faltan SMTP_HOST, SMTP_USER, SMTP_PASS o MAIL_FROM")
	}
	return c, nil
}

// SMTPMailer envía los correos por SMTP con STARTTLS.
type SMTPMailer struct {
	cfg SMTPConfig
}

func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer { return &SMTPMailer{cfg: cfg} }

func (m *SMTPMailer) EnviarPinRecuperacion(ctx context.Context, _ uuid.UUID, email, pin string) error {
	cuerpo := fmt.Sprintf(
		"Tu código de recuperación de CimAlert es: %s\r\n\r\n"+
			"Vence en %d minutos. Si no lo solicitaste, ignora este mensaje.\r\n"+
			"Nunca compartas este código con nadie.\r\n",
		pin, pinTTLMinutos,
	)
	return m.enviar(ctx, email, "Tu código de recuperación", cuerpo)
}

func (m *SMTPMailer) EnviarAvisoPasswordCambiado(ctx context.Context, _ uuid.UUID, email string) error {
	cuerpo := "La contraseña de tu cuenta de CimAlert fue cambiada.\r\n\r\n" +
		"Si no fuiste tú, contacta al administrador lo antes posible.\r\n"
	return m.enviar(ctx, email, "Tu contraseña fue cambiada", cuerpo)
}

func (m *SMTPMailer) EnviarAvisoCuentaVinculadaGoogle(ctx context.Context, _ uuid.UUID, email string) error {
	cuerpo := avisoVinculoGoogle + "\r\n\r\n" +
		"Si no fuiste tú, contacta al administrador lo antes posible.\r\n"
	return m.enviar(ctx, email, "Tu cuenta fue vinculada con Google", cuerpo)
}

func (m *SMTPMailer) enviar(ctx context.Context, destino, asunto, cuerpo string) error {
	// Evita inyección de cabeceras: el destino debe ser una sola dirección válida.
	if strings.ContainsAny(destino, "\r\n") {
		return fmt.Errorf("destinatario inválido")
	}
	to, err := mail.ParseAddress(destino)
	if err != nil {
		return fmt.Errorf("destinatario inválido: %w", err)
	}
	from := mail.Address{Name: m.cfg.Name, Address: m.cfg.From}

	var body bytes.Buffer
	fmt.Fprintf(&body, "From: %s\r\n", from.String())
	fmt.Fprintf(&body, "To: %s\r\n", to.String())
	fmt.Fprintf(&body, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", asunto))
	fmt.Fprintf(&body, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	body.WriteString("MIME-Version: 1.0\r\n")
	body.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	body.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	qp := quotedprintable.NewWriter(&body)
	if _, err := qp.Write([]byte(cuerpo)); err != nil {
		return err
	}
	if err := qp.Close(); err != nil {
		return err
	}

	return m.entregar(ctx, to.Address, body.Bytes())
}

// entregar hace la conexión a mano (en vez de smtp.SendMail) para respetar el ctx.
func (m *SMTPMailer) entregar(ctx context.Context, destino string, msg []byte) error {
	addr := net.JoinHostPort(m.cfg.Host, m.cfg.Port)

	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp conectar: %w", err)
	}
	// Si el ctx vence, cierra la conexión para desbloquear cualquier operación.
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp cliente: %w", err)
	}
	defer c.Close()

	if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("smtp starttls: %w", err)
	}
	if err := c.Auth(smtp.PlainAuth("", m.cfg.User, m.cfg.Pass, m.cfg.Host)); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := c.Mail(m.cfg.From); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := c.Rcpt(destino); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp escribir: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp cerrar data: %w", err)
	}
	return c.Quit()
}
