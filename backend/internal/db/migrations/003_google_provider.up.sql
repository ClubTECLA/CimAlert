-- Una identidad de Google (provider_id = sub) corresponde a una sola cuenta.
-- Sin este índice, dos inicios de sesión concurrentes con el mismo sub podrían
-- crear dos cuentas o vincular el mismo sub a dos usuarios.
CREATE UNIQUE INDEX auth_providers_provider_provider_id_key ON public.auth_providers (provider, provider_id);
