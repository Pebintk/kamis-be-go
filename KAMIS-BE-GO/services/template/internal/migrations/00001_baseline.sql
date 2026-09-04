-- Baseline schema for the template service's sample Resource domain.
--
-- When you copy this tree to start a service, replace this file with your
-- own baseline and renumber from 00001.

-- +goose Up
CREATE TABLE public.resources (
    id bigint NOT NULL,
    name text NOT NULL,
    quantity bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);
CREATE SEQUENCE public.resources_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.resources_id_seq OWNED BY public.resources.id;
ALTER TABLE ONLY public.resources ALTER COLUMN id SET DEFAULT nextval('public.resources_id_seq'::regclass);
ALTER TABLE ONLY public.resources
    ADD CONSTRAINT resources_pkey PRIMARY KEY (id);

-- +goose Down
DROP TABLE IF EXISTS public.resources;
