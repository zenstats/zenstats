-- Extensions
CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;
COMMENT ON EXTENSION citext IS 'data type for case-insensitive character strings';

-- Enums
CREATE TYPE public.billing_interval AS ENUM ('monthly', 'yearly', 'weekly', 'daily');
CREATE TYPE public.site_membership_role AS ENUM ('owner', 'admin', 'viewer');

-- Users table
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
    is_admin boolean DEFAULT false NOT NULL,
    created_at timestamp(0) without time zone NOT NULL,
    updated_at timestamp(0) without time zone NOT NULL
);
CREATE UNIQUE INDEX idx_users_email ON public.users(email);

-- User sessions
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

-- API keys
CREATE TABLE public.api_keys (
    id bigserial PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    name character varying(255) NOT NULL,
    key_hash character varying(255) NOT NULL UNIQUE,
    created_at timestamp(0) without time zone NOT NULL
);
CREATE INDEX api_keys_key_hash_index ON public.api_keys USING btree (key_hash);

-- Sites
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

-- Site memberships
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

-- Funnel steps
CREATE TABLE public.funnel_steps (
    id bigserial PRIMARY KEY,
    funnel_id bigint NOT NULL REFERENCES public.funnels(id) ON DELETE CASCADE,
    goal_id bigint NOT NULL REFERENCES public.goals(id) ON DELETE CASCADE,
    step_order integer NOT NULL,
    UNIQUE (funnel_id, goal_id)
);

-- Search engines (global)
CREATE TABLE public.search_engines (
    id bigserial PRIMARY KEY,
    domain VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL
);

-- Shield rules - IP
CREATE TABLE public.shield_rules_ip (
    id bigserial PRIMARY KEY,
    site_id bigint NOT NULL REFERENCES public.sites(id) ON DELETE CASCADE,
    ip_address VARCHAR(45) NOT NULL,
    description TEXT,
    added_by VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    UNIQUE (site_id, ip_address)
);

-- Shield rules - Hostname
CREATE TABLE public.shield_rules_hostname (
    id bigserial PRIMARY KEY,
    site_id bigint NOT NULL REFERENCES public.sites(id) ON DELETE CASCADE,
    hostname VARCHAR(255) NOT NULL,
    description TEXT,
    added_by VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    UNIQUE (site_id, hostname)
);

-- Shield rules - Country
CREATE TABLE public.shield_rules_country (
    id bigserial PRIMARY KEY,
    site_id bigint NOT NULL REFERENCES public.sites(id) ON DELETE CASCADE,
    country_code VARCHAR(2) NOT NULL,
    description TEXT,
    added_by VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    UNIQUE (site_id, country_code)
);

-- User groups (subscription plans)
CREATE TABLE public.user_groups (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    max_sites INT DEFAULT 3 NOT NULL,
    max_monthly_events INT DEFAULT 10000 NOT NULL,
    max_api_keys INT DEFAULT 2 NOT NULL,
    max_sub_accounts INT DEFAULT 0 NOT NULL,
    custom_search_engines BOOLEAN DEFAULT false NOT NULL,
    is_default BOOLEAN DEFAULT false NOT NULL,
    price DECIMAL(10,2) DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_user_groups_name ON public.user_groups(name);

-- User configs (user subscription status)
CREATE TABLE public.user_configs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE REFERENCES public.users(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES public.user_groups(id),
    status VARCHAR(20) DEFAULT 'active' NOT NULL,
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX idx_user_configs_user_id ON public.user_configs(user_id);
CREATE INDEX idx_user_configs_group_id ON public.user_configs(group_id);

-- Custom search engines (per user)
CREATE TABLE public.custom_search_engines (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    domain VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_custom_search_engines_user_domain ON public.custom_search_engines(user_id, domain);
CREATE INDEX idx_custom_search_engines_user_id ON public.custom_search_engines(user_id);

-- Sub accounts
CREATE TABLE public.sub_accounts (
    id BIGSERIAL PRIMARY KEY,
    parent_user_id BIGINT NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(100),
    role VARCHAR(20) DEFAULT 'viewer' NOT NULL,
    status VARCHAR(20) DEFAULT 'active' NOT NULL,
    last_seen TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX idx_sub_accounts_parent_user_id ON public.sub_accounts(parent_user_id);

-- Password reset tokens
CREATE TABLE public.password_reset_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    token VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used BOOLEAN DEFAULT false NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_password_reset_tokens_token ON public.password_reset_tokens(token);
CREATE INDEX idx_password_reset_tokens_user_id ON public.password_reset_tokens(user_id);
CREATE INDEX idx_password_reset_tokens_expires_at ON public.password_reset_tokens(expires_at);

-- Shared links (embeddable public dashboard links)
CREATE TABLE public.shared_links (
    id BIGSERIAL PRIMARY KEY,
    site_id bigint NOT NULL REFERENCES public.sites(id) ON DELETE CASCADE,
    name character varying(255) NOT NULL,
    slug character varying(64) NOT NULL UNIQUE,
    password_hash character varying(255),
    created_at timestamp(0) without time zone NOT NULL DEFAULT now(),
    updated_at timestamp(0) without time zone NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_shared_links_site_name ON public.shared_links(site_id, name);
CREATE INDEX idx_shared_links_slug ON public.shared_links(slug);

-- Segments (saved filter sets)
CREATE TABLE public.segments (
    id BIGSERIAL PRIMARY KEY,
    site_id bigint NOT NULL REFERENCES public.sites(id) ON DELETE CASCADE,
    name character varying(255) NOT NULL,
    filters text NOT NULL DEFAULT '[]',
    created_by bigint REFERENCES public.users(id) ON DELETE SET NULL,
    created_at timestamp(0) without time zone NOT NULL DEFAULT now(),
    updated_at timestamp(0) without time zone NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_segments_site_name ON public.segments(site_id, name);
CREATE INDEX idx_segments_site_id ON public.segments(site_id);

-- Monthly event counts (quota tracking)
CREATE TABLE public.monthly_event_counts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    year INT NOT NULL,
    month INT NOT NULL,
    count BIGINT DEFAULT 0 NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_monthly_event_counts_user_year_month ON public.monthly_event_counts(user_id, year, month);
