-- Baseline schema for the project service.
--
-- Generated from the GORM models by running AutoMigrate against an empty
-- PostgreSQL 16 database and dumping the result, so the first migration is
-- exactly the schema the service ran with before migrations existed.

-- +goose Up
CREATE TABLE public.log_projects (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    project_id text,
    username text NOT NULL,
    action character varying(1000) NOT NULL,
    action_date timestamp with time zone NOT NULL
);
CREATE TABLE public.project_asset_usages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    project_id text NOT NULL,
    plat_nomor text NOT NULL,
    tipe_aset text NOT NULL,
    asset_use_cost bigint NOT NULL,
    asset_fuel_cost bigint NOT NULL
);
CREATE TABLE public.project_resource_usages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    project_id text NOT NULL,
    resource_id text NOT NULL,
    sell_price bigint NOT NULL,
    quantity_used bigint NOT NULL
);
CREATE TABLE public.projects (
    id text NOT NULL,
    project_type boolean NOT NULL,
    project_status bigint NOT NULL,
    project_payment_status bigint,
    project_name text NOT NULL,
    project_description text,
    project_client_id text NOT NULL,
    project_client_name text NOT NULL,
    project_delivery_address text NOT NULL,
    created_date timestamp with time zone,
    project_start_date timestamp with time zone,
    project_end_date timestamp with time zone,
    project_payment_date timestamp with time zone,
    project_total_pemasukkan bigint,
    project_pickup_address text,
    project_phl_count bigint,
    project_phl_pay bigint,
    project_total_pengeluaran bigint
);
ALTER TABLE ONLY public.log_projects
    ADD CONSTRAINT log_projects_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.project_asset_usages
    ADD CONSTRAINT project_asset_usages_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.project_resource_usages
    ADD CONSTRAINT project_resource_usages_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_pkey PRIMARY KEY (id);
CREATE INDEX idx_log_projects_project_id ON public.log_projects USING btree (project_id);
CREATE INDEX idx_project_asset_usages_project_id ON public.project_asset_usages USING btree (project_id);
CREATE INDEX idx_project_resource_usages_project_id ON public.project_resource_usages USING btree (project_id);
CREATE INDEX idx_projects_project_client_id ON public.projects USING btree (project_client_id);
CREATE INDEX idx_projects_project_end_date ON public.projects USING btree (project_end_date);
CREATE INDEX idx_projects_project_payment_status ON public.projects USING btree (project_payment_status);
CREATE INDEX idx_projects_project_start_date ON public.projects USING btree (project_start_date);
CREATE INDEX idx_projects_project_status ON public.projects USING btree (project_status);
CREATE INDEX idx_projects_project_type ON public.projects USING btree (project_type);

-- +goose Down
DROP TABLE IF EXISTS public.projects;
DROP TABLE IF EXISTS public.project_resource_usages;
DROP TABLE IF EXISTS public.project_asset_usages;
DROP TABLE IF EXISTS public.log_projects;
