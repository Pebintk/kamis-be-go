-- +goose Up
CREATE TABLE widgets (
    id bigserial PRIMARY KEY,
    name text NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS widgets;
