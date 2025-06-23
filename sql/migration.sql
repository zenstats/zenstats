-- Extensions
CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;
COMMENT ON EXTENSION citext IS 'data type for case-insensitive character strings';

-- Enums
CREATE TYPE public.billing_interval AS ENUM ('monthly', 'yearly', 'weekly', 'daily');
CREATE TYPE public.site_membership_role AS ENUM ('owner', 'admin', 'viewer');


CREATE TABLE public.user_sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id bigint NOT NULL,
    token bytea NOT NULL,
    device character varying(255) NOT NULL,
    last_used_at timestamp(0) without time zone NOT NULL,
    timeout_at timestamp(0) without time zone NOT NULL,
    created_at timestamp(0) without time zone NOT NULL
);
CREATE INDEX idx_user_sessions_token ON public.user_sessions(token);

CREATE TABLE public.users (
    id BIGSERIAL PRIMARY KEY,
    email public.citext NOT NULL,
    email_verified boolean DEFAULT false NOT NULL,
    name character varying(255),
    last_seen timestamp(0) without time zone DEFAULT now(),
    password_hash character varying(255),
    previous_email public.citext,
    totp_secret bytea,
    totp_enabled boolean DEFAULT false NOT NULL,
    totp_last_used_at timestamp(0) without time zone,
    totp_token character varying(255),
    notes text,
    created_at timestamp(0) without time zone NOT NULL,
    updated_at timestamp(0) without time zone NOT NULL
);
CREATE UNIQUE INDEX idx_users_email ON public.users(email);

CREATE TABLE public.api_keys (
    id bigserial PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    name character varying(255) NOT NULL,
    key_hash character varying(255) NOT NULL UNIQUE,
    created_at timestamp(0) without time zone NOT NULL
);
CREATE INDEX api_keys_key_hash_index ON public.api_keys USING btree (key_hash);

CREATE TABLE public.sites (
    id BIGSERIAL PRIMARY KEY,
    domain VARCHAR(255) NOT NULL UNIQUE,
    timezone VARCHAR(50) NOT NULL DEFAULT 'UTC',
    public BOOLEAN DEFAULT false NOT NULL,
    stats_start_date DATE,
    ingest_rate_limit_scale_seconds integer DEFAULT 60 NOT NULL,
    ingest_limit_per_minute INTEGER DEFAULT 1000,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX idx_sites_domain ON public.sites(domain);

CREATE TABLE public.site_memberships (
    id BIGSERIAL PRIMARY KEY,
    site_id bigint NOT NULL REFERENCES public.sites(id) ON DELETE CASCADE,
    user_id bigint NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    role public.site_membership_role DEFAULT 'owner' NOT NULL,
    UNIQUE (site_id, user_id)
);

-- Goals (Conversion tracking)
CREATE TABLE public.goals (
    id bigserial PRIMARY KEY,
    site_id bigint NOT NULL REFERENCES public.sites(id) ON DELETE CASCADE,
    event_name text,
    page_path text,
    display_name text NOT NULL,
    UNIQUE (site_id, display_name)
);

-- Funnels (Conversion paths)
CREATE TABLE public.funnels (
    id bigserial PRIMARY KEY,
    site_id bigint NOT NULL REFERENCES public.sites(id) ON DELETE CASCADE,
    name character varying(255) NOT NULL,
    UNIQUE (site_id, name)
);

-- Funnel Steps
CREATE TABLE public.funnel_steps (
    id bigserial PRIMARY KEY,
    funnel_id bigint NOT NULL REFERENCES public.funnels(id) ON DELETE CASCADE,
    goal_id bigint NOT NULL REFERENCES public.goals(id) ON DELETE CASCADE,
    step_order integer NOT NULL,
    UNIQUE (funnel_id, goal_id)
