-- +goose Up
-- +goose StatementBegin

-- resource-Spalte für alerts (z.B. Mount-Point bei Disk-Alerts)
ALTER TABLE alerts ADD COLUMN resource TEXT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite unterstützt kein DROP COLUMN in älteren Versionen — Migration ist irreversibel
SELECT 1;
-- +goose StatementEnd
