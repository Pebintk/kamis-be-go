-- Baseline schema for the profile service.
--
-- Generated from the GORM models by running AutoMigrate against an empty
-- PostgreSQL 16 database and dumping the result, so the first migration is
-- exactly the schema the service ran with before migrations existed.

-- +goose Up
CREATE TABLE public.clients (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name_client text NOT NULL,
    no_telp_client text NOT NULL,
    email_client text NOT NULL,
    type_client boolean NOT NULL,
    company_client text,
    address_client text NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);
CREATE TABLE public.end_users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    username text NOT NULL,
    email text NOT NULL,
    password text NOT NULL,
    user_type text NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);
CREATE TABLE public.supplier_assets (
    supplier_id uuid,
    asset_id text
);
CREATE TABLE public.supplier_purchases (
    supplier_id uuid,
    purchase_id text
);
CREATE TABLE public.supplier_resources (
    supplier_id uuid,
    resource_id bigint
);
CREATE TABLE public.suppliers (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name_supplier text NOT NULL,
    no_telp_supplier text NOT NULL,
    email_supplier text NOT NULL,
    company_supplier text,
    address_supplier text NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);
ALTER TABLE ONLY public.clients
    ADD CONSTRAINT clients_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.end_users
    ADD CONSTRAINT end_users_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.suppliers
    ADD CONSTRAINT suppliers_pkey PRIMARY KEY (id);
CREATE UNIQUE INDEX idx_clients_email_client ON public.clients USING btree (email_client);
CREATE UNIQUE INDEX idx_clients_name_client ON public.clients USING btree (name_client);
CREATE UNIQUE INDEX idx_clients_no_telp_client ON public.clients USING btree (no_telp_client);
CREATE UNIQUE INDEX idx_end_users_email ON public.end_users USING btree (email);
CREATE UNIQUE INDEX idx_end_users_username ON public.end_users USING btree (username);
CREATE INDEX idx_supplier_assets_supplier_id ON public.supplier_assets USING btree (supplier_id);
CREATE INDEX idx_supplier_purchases_supplier_id ON public.supplier_purchases USING btree (supplier_id);
CREATE INDEX idx_supplier_resources_supplier_id ON public.supplier_resources USING btree (supplier_id);
CREATE UNIQUE INDEX idx_suppliers_email_supplier ON public.suppliers USING btree (email_supplier);
CREATE UNIQUE INDEX idx_suppliers_name_supplier ON public.suppliers USING btree (name_supplier);
CREATE UNIQUE INDEX idx_suppliers_no_telp_supplier ON public.suppliers USING btree (no_telp_supplier);

-- +goose Down
DROP TABLE IF EXISTS public.suppliers;
DROP TABLE IF EXISTS public.supplier_resources;
DROP TABLE IF EXISTS public.supplier_purchases;
DROP TABLE IF EXISTS public.supplier_assets;
DROP TABLE IF EXISTS public.end_users;
DROP TABLE IF EXISTS public.clients;
