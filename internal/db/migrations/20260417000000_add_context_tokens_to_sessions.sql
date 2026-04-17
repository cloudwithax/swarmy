-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN context_prompt_tokens INTEGER NOT NULL DEFAULT 0 CHECK (context_prompt_tokens >= 0);
ALTER TABLE sessions ADD COLUMN context_completion_tokens INTEGER NOT NULL DEFAULT 0 CHECK (context_completion_tokens >= 0);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN context_prompt_tokens;
ALTER TABLE sessions DROP COLUMN context_completion_tokens;
-- +goose StatementEnd
