-- name: CreateSesion :exec
-- Las fechas vienen del reloj de la API, como password_changed_at.
INSERT INTO
    refresh_tokens (user_id, token_hash, expires_at, pwd_at, remember_me, created_at, last_used_at)
VALUES
    (@user_id, @token_hash, @expires_at, @pwd_at, @remember_me, @ahora, @ahora);

-- name: GetSesionPorTokenHash :one
-- Encuentra la sesión por su token actual o por el anterior (ya rotado).
SELECT
    *
FROM
    refresh_tokens
WHERE
    token_hash = $1
    OR prev_token_hash = $1
LIMIT 1;

-- name: RotarSesion :execrows
-- Condicional: de dos refresh concurrentes con el mismo token solo uno rota;
-- el otro ve 0 filas y entra por la ventana de gracia.
UPDATE
    refresh_tokens
SET
    prev_token_hash = token_hash,
    token_hash = @nuevo,
    expires_at = @expires_at,
    last_used_at = @ahora
WHERE
    id = @id
    AND token_hash = @anterior
    AND revoked_at IS NULL;

-- name: RevocarSesionPorID :exec
UPDATE
    refresh_tokens
SET
    revoked_at = now()
WHERE
    id = $1
    AND revoked_at IS NULL;

-- name: RevocarSesionPorTokenHash :exec
UPDATE
    refresh_tokens
SET
    revoked_at = now()
WHERE
    (token_hash = $1 OR prev_token_hash = $1)
    AND revoked_at IS NULL;

-- name: RevocarSesionesPorUserID :exec
UPDATE
    refresh_tokens
SET
    revoked_at = now()
WHERE
    user_id = $1
    AND revoked_at IS NULL;

-- name: GetUserConRolPorID :one
-- El usuario de la sesión, para validarlo y firmar el token de acceso nuevo.
SELECT
    sqlc.embed(u),
    r.nombre AS rol_name
FROM
    users u
    JOIN roles r ON r.id = u.rol_id
WHERE
    u.id = $1;
