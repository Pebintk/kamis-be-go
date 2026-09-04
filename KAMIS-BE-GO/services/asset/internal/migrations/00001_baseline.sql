-- Baseline schema for the asset service.
--
-- Generated from the GORM models by running AutoMigrate against an empty
-- PostgreSQL 16 database and dumping the result, so the first migration is
-- exactly the schema the service ran with before migrations existed.

-- +goose Up
CREATE TABLE public.asset_reservations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    plat_nomor text NOT NULL,
    project_id text NOT NULL,
    start_date timestamp with time zone NOT NULL,
    end_date timestamp with time zone NOT NULL,
    reservation_status text NOT NULL
);
CREATE TABLE public.assets (
    plat_nomor text NOT NULL,
    nama text NOT NULL,
    jenis_aset text NOT NULL,
    status text NOT NULL,
    tanggal_perolehan timestamp with time zone,
    nilai_perolehan bigint NOT NULL,
    deskripsi text NOT NULL,
    id_supplier uuid,
    foto_key text,
    foto_content_type text,
    deleted_at timestamp with time zone
);
CREATE TABLE public.maintenances (
    id bigint NOT NULL,
    tanggal_mulai_maintenance timestamp with time zone NOT NULL,
    tanggal_selesai_maintenance timestamp with time zone,
    deskripsi_pekerjaan text NOT NULL,
    biaya numeric NOT NULL,
    status text NOT NULL,
    asset_plat_nomor text NOT NULL
);
CREATE SEQUENCE public.maintenances_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.maintenances_id_seq OWNED BY public.maintenances.id;
ALTER TABLE ONLY public.maintenances ALTER COLUMN id SET DEFAULT nextval('public.maintenances_id_seq'::regclass);
ALTER TABLE ONLY public.asset_reservations
    ADD CONSTRAINT asset_reservations_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.assets
    ADD CONSTRAINT assets_pkey PRIMARY KEY (plat_nomor);
ALTER TABLE ONLY public.maintenances
    ADD CONSTRAINT maintenances_pkey PRIMARY KEY (id);
CREATE INDEX idx_asset_reservations_plat_nomor ON public.asset_reservations USING btree (plat_nomor);
CREATE INDEX idx_asset_reservations_project_id ON public.asset_reservations USING btree (project_id);
CREATE INDEX idx_asset_reservations_reservation_status ON public.asset_reservations USING btree (reservation_status);
CREATE INDEX idx_assets_deleted_at ON public.assets USING btree (deleted_at);
CREATE INDEX idx_assets_id_supplier ON public.assets USING btree (id_supplier);
CREATE INDEX idx_maintenances_asset_plat_nomor ON public.maintenances USING btree (asset_plat_nomor);

-- +goose Down
DROP TABLE IF EXISTS public.maintenances;
DROP TABLE IF EXISTS public.assets;
DROP TABLE IF EXISTS public.asset_reservations;
