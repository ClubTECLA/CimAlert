-- Una identidad de Google (provider_id = sub) corresponde a una sola cuenta.
-- Sin este índice, dos inicios de sesión concurrentes con el mismo sub podrían
-- crear dos cuentas o vincular el mismo sub a dos usuarios.
CREATE UNIQUE INDEX auth_providers_provider_provider_id_key ON public.auth_providers (provider, provider_id);

-- Sesiones con refresh token: una fila por sesión, que rota token_hash en cada
-- /auth/refresh. prev_token_hash es el token anterior, para tolerar dos
-- refresh concurrentes (dos pestañas) y detectar el reuso de uno ya rotado.
-- pwd_at es la versión de la contraseña con la que se abrió la sesión, en el
-- mismo formato que el claim del JWT (middleware.VersionCredencial).
-- created_at es el inicio de la sesión: la rotación no lo cambia.
ALTER TABLE public.refresh_tokens
    ADD COLUMN prev_token_hash text,
    ADD COLUMN pwd_at text DEFAULT '' NOT NULL,
    ADD COLUMN remember_me boolean DEFAULT false NOT NULL,
    ADD COLUMN last_used_at timestamptz DEFAULT now() NOT NULL;

CREATE INDEX idx_refresh_tokens_prev_token_hash ON public.refresh_tokens USING btree (prev_token_hash);
