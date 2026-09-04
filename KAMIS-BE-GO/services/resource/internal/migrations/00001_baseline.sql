-- Baseline schema for the resource service.
--
-- Generated from the GORM models by running AutoMigrate against an empty
-- PostgreSQL 16 database and dumping the result, so the first migration is
-- exactly the schema the service ran with before migrations existed.

-- +goose Up
CREATE TABLE public.resource_suppliers (
    resource_id bigint NOT NULL,
    supplier_id uuid NOT NULL
);
CREATE TABLE public.resources (
    id bigint NOT NULL,
    resource_name text NOT NULL,
    resource_description text NOT NULL,
    resource_stock bigint NOT NULL,
    resource_price bigint NOT NULL
);
CREATE SEQUENCE public.resources_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.resources_id_seq OWNED BY public.resources.id;
ALTER TABLE ONLY public.resources ALTER COLUMN id SET DEFAULT nextval('public.resources_id_seq'::regclass);
ALTER TABLE ONLY public.resource_suppliers
    ADD CONSTRAINT resource_suppliers_pkey PRIMARY KEY (resource_id, supplier_id);
ALTER TABLE ONLY public.resources
    ADD CONSTRAINT resources_pkey PRIMARY KEY (id);
CREATE UNIQUE INDEX idx_resources_resource_name ON public.resources USING btree (resource_name);

-- +goose Down
DROP TABLE IF EXISTS public.resources;
DROP TABLE IF EXISTS public.resource_suppliers;
