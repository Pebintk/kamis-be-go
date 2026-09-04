-- Baseline schema for the finance service.
--
-- Generated from the GORM models by running AutoMigrate against an empty
-- PostgreSQL 16 database and dumping the result, so the first migration is
-- exactly the schema the service ran with before migrations existed.

-- +goose Up
CREATE TABLE public.lapkeus (
    id text NOT NULL,
    activity_type bigint NOT NULL,
    pemasukan bigint,
    pengeluaran bigint,
    description text,
    payment_date timestamp with time zone
);
ALTER TABLE ONLY public.lapkeus
    ADD CONSTRAINT lapkeus_pkey PRIMARY KEY (id);
CREATE INDEX idx_lapkeus_activity_type ON public.lapkeus USING btree (activity_type);
CREATE INDEX idx_lapkeus_payment_date ON public.lapkeus USING btree (payment_date);

-- +goose Down
DROP TABLE IF EXISTS public.lapkeus;
