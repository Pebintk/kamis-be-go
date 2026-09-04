-- +goose Up
ALTER TABLE widgets ADD COLUMN colour text;

-- +goose Down
ALTER TABLE widgets DROP COLUMN colour;
