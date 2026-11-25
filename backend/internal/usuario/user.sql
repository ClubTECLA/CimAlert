-- name: CreateUser :one
INSERT INTO
    users (email, username, rol_id, password_hash)
VALUES
    ($1, $2, $3, $4) RETURNING *;

-- Se inserta junto con el usuario dentro de la misma transacción, para que
-- nunca quede un usuario sin forma de autenticarse ni un auth_provider 
-- sin padre.
-- name: CreateLocalAuthProvider :exec
INSERT INTO
    auth_providers (user_id, provider)
VALUES
    ($1, 'local');

-- name: GetUserByEmail :one
SELECT
    *
FROM
    users
WHERE
    email = $1;

-- name: GetUserByID :one
SELECT
    *
FROM
    users
WHERE
    id = $1;

-- name: GetUserWithRolByID :one
SELECT
    u.*,
    r.nombre AS rol_name
FROM
    users u
    JOIN roles r ON r.id = u.rol_id
WHERE
    u.id = $1;

-- name: ListAllUsers :many
SELECT
    u.id,
    u.username,
    u.email,
    r.nombre AS rol_name,
    COUNT(ur.reporte_id)::int AS reportes_count
FROM
    users u
    JOIN roles r ON r.id = u.rol_id
    LEFT JOIN users_reports ur ON ur.user_id = u.id
GROUP BY
    u.id,
    r.nombre
ORDER BY
    u.username DESC;

-- name: CreatePIN :exec
INSERT INTO
    recovery_pins (user_id, pin_hash, expires_at)
VALUES
    ($1, $2, $3);

-- name: GetLatestActivePINByUserID :one
SELECT
    *
FROM
    recovery_pins
WHERE
    user_id = $1
    AND expires_at > now()
    AND used_at IS NULL
    AND verified_at IS NULL
ORDER BY
    created_at DESC
LIMIT 1;

-- name: SetPinAsVerifiedByID :execrows
-- Condicional para que de dos verificaciones concurrentes del mismo PIN solo
-- una emita token.
UPDATE
    recovery_pins
SET
    verified_at = now(),
    reset_token_hash = $2,
    reset_expires_at = $3
WHERE
    id = $1
    AND used_at IS NULL
    AND verified_at IS NULL
    AND expires_at > clock_timestamp();

-- name: GetActiveResetByTokenHash :one
SELECT
    id,
    user_id
FROM
    recovery_pins
WHERE
    reset_token_hash = $1
    AND used_at IS NULL
    AND reset_expires_at > now();

-- name: LockUserByID :exec
-- Serializa por cuenta la emisión de PIN: la segunda petición espera aquí hasta que la primera termine.
SELECT
    id
FROM
    users
WHERE
    id = $1
FOR UPDATE;

-- name: CountPINsLastHourByUserID :one
SELECT
    count(*)
FROM
    recovery_pins
WHERE
    user_id = $1
    AND created_at > now() - interval '1 hour';

-- name: AddPINAttemptByPINID :one
UPDATE
    recovery_pins
SET
    attempts = attempts + 1
WHERE
    id = $1
RETURNING attempts;

-- name: CancelPINByUserID :exec
-- Sin filtrar por expires_at: un PIN verificado puede haber expirado y
-- tener todavía un token de reset vivo, que también hay que cancelar.
UPDATE
    recovery_pins
SET
    used_at = now()
WHERE
    user_id = $1
    AND used_at IS NULL;

-- name: DeleteUsedOrExpiredPINByUserID :exec
DELETE FROM
    recovery_pins
WHERE
    user_id = $1
    AND created_at < now() - interval '1 hour'
    AND (used_at IS NOT NULL OR expires_at < now());

-- name: SetNewPasswordByUserID :exec
-- password_changed_at solo se escribe en dos lugares: un flujo que probó el
-- control del buzón (recuperación) y el vínculo con Google que quita el
-- password (RemovePasswordByUserID). El vínculo con Google cuenta con que NULL
-- signifique "nunca probado".
UPDATE
    users
SET
    password_hash = $1,
    -- Viene de la API, no de now(): se compara contra el iat del JWT, que
    -- firma la API con su reloj.
    password_changed_at = $2
WHERE
    id = $3;

-- name: ConsumeResetByID :execrows
-- Cierra el token de reset; 0 filas si ya se usó, se canceló o expiró.
UPDATE
    recovery_pins
SET
    used_at = now()
WHERE
    id = $1
    AND used_at IS NULL
    AND verified_at IS NOT NULL
    AND reset_expires_at > clock_timestamp();

-- name: GetPasswordChangedAtByUserID :one
SELECT
    password_changed_at
FROM
    users
WHERE
    id = $1;

-- name: GetUserWithRolByGoogleSub :one
SELECT
    sqlc.embed(u),
    r.nombre AS rol_name
FROM
    users u
    JOIN auth_providers ap ON ap.user_id = u.id
    JOIN roles r ON r.id = u.rol_id
WHERE
    ap.provider = 'google'
    AND ap.provider_id = $1;

-- name: GetUserWithRolByEmailForUpdate :one
-- El bloqueo serializa los intentos concurrentes de vincular la misma cuenta.
SELECT
    sqlc.embed(u),
    r.nombre AS rol_name
FROM
    users u
    JOIN roles r ON r.id = u.rol_id
WHERE
    u.email = $1
FOR UPDATE OF u;

-- name: GetGoogleProviderIDByUserID :one
-- Sin filas: el usuario no está vinculado con Google.
SELECT
    provider_id
FROM
    auth_providers
WHERE
    user_id = $1
    AND provider = 'google';

-- name: CreateGoogleAuthProvider :exec
INSERT INTO
    auth_providers (user_id, provider, provider_id)
VALUES
    ($1, 'google', $2);

-- name: RemovePasswordByUserID :one
-- password_changed_at solo se escribe en dos lugares: un flujo que probó el
-- control del buzón (recuperación) y este vínculo con Google. Como en
-- SetNewPasswordByUserID, $2 viene del reloj de la API.
UPDATE
    users
SET
    password_hash = NULL,
    password_changed_at = $2,
    updated_at = now()
WHERE
    id = $1
RETURNING *;

-- name: DeleteLocalAuthProviderByUserID :exec
DELETE FROM
    auth_providers
WHERE
    user_id = $1
    AND provider = 'local';