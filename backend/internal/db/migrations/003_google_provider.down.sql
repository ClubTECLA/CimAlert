DROP INDEX IF EXISTS public.idx_refresh_tokens_prev_token_hash;
ALTER TABLE public.refresh_tokens
    DROP COLUMN IF EXISTS prev_token_hash,
    DROP COLUMN IF EXISTS pwd_at,
    DROP COLUMN IF EXISTS remember_me,
    DROP COLUMN IF EXISTS last_used_at;

DROP INDEX IF EXISTS public.auth_providers_provider_provider_id_key;
