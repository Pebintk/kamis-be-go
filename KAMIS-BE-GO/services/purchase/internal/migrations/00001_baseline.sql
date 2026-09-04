-- Baseline schema for the purchase service.
--
-- Generated from the GORM models by running AutoMigrate against an empty
-- PostgreSQL 16 database and dumping the result, so the first migration is
-- exactly the schema the service ran with before migrations existed.

-- +goose Up
CREATE TABLE public.asset_temps (
    id bigint NOT NULL,
    asset_name text NOT NULL,
    asset_description text NOT NULL,
    asset_type text NOT NULL,
    asset_price bigint NOT NULL,
    foto_key text,
    foto_content_type text
);
CREATE SEQUENCE public.asset_temps_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.asset_temps_id_seq OWNED BY public.asset_temps.id;
CREATE TABLE public.log_purchases (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    purchase_id text,
    username text NOT NULL,
    action character varying(1000) NOT NULL,
    action_date timestamp with time zone NOT NULL
);
CREATE TABLE public.purchases (
    id text NOT NULL,
    purchase_supplier uuid NOT NULL,
    purchase_type boolean NOT NULL,
    purchase_status text NOT NULL,
    purchase_price bigint NOT NULL,
    purchase_note text NOT NULL,
    purchase_submission_date timestamp with time zone NOT NULL,
    purchase_update_date timestamp with time zone NOT NULL,
    purchase_asset bigint,
    purchase_payment_date timestamp with time zone
);
CREATE TABLE public.resource_temps (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    purchase_id text,
    resource_id bigint NOT NULL,
    resource_name text NOT NULL,
    resource_total bigint NOT NULL,
    resource_price bigint NOT NULL,
    deleted_at timestamp with time zone
);
ALTER TABLE ONLY public.asset_temps ALTER COLUMN id SET DEFAULT nextval('public.asset_temps_id_seq'::regclass);
ALTER TABLE ONLY public.asset_temps
    ADD CONSTRAINT asset_temps_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.log_purchases
    ADD CONSTRAINT log_purchases_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.purchases
    ADD CONSTRAINT purchases_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.resource_temps
    ADD CONSTRAINT resource_temps_pkey PRIMARY KEY (id);
CREATE INDEX idx_log_purchases_purchase_id ON public.log_purchases USING btree (purchase_id);
CREATE INDEX idx_purchases_purchase_status ON public.purchases USING btree (purchase_status);
CREATE INDEX idx_purchases_purchase_submission_date ON public.purchases USING btree (purchase_submission_date);
CREATE INDEX idx_purchases_purchase_supplier ON public.purchases USING btree (purchase_supplier);
CREATE INDEX idx_resource_temps_deleted_at ON public.resource_temps USING btree (deleted_at);
CREATE INDEX idx_resource_temps_purchase_id ON public.resource_temps USING btree (purchase_id);

-- +goose Down
DROP TABLE IF EXISTS public.resource_temps;
DROP TABLE IF EXISTS public.purchases;
DROP TABLE IF EXISTS public.log_purchases;
DROP TABLE IF EXISTS public.asset_temps;
