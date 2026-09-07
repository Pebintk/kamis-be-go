-- +goose Up
-- An account can hold several roles. user_type stays the primary one — it is
-- what the JWT's `role` claim and the API's `role` field carry, and what the
-- frontend's dashboard redirect reads — while user_roles holds the full set,
-- primary included.
CREATE TABLE user_roles (
    user_id       UUID NOT NULL REFERENCES end_users (id) ON DELETE CASCADE,
    discriminator TEXT NOT NULL,
    PRIMARY KEY (user_id, discriminator)
);

CREATE INDEX idx_user_roles_user_id ON user_roles (user_id);

-- Backfill: every existing account holds exactly the role it already had, so
-- the set and the primary agree from the first moment.
INSERT INTO user_roles (user_id, discriminator)
SELECT id, user_type FROM end_users;

-- +goose Down
DROP TABLE user_roles;
