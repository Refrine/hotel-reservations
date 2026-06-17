-- +goose Up
CREATE TABLE IF NOT EXISTS bookings (
    id          BIGSERIAL    PRIMARY KEY,
    status      VARCHAR(30)  NOT NULL DEFAULT 'awaits_confirmation',
    user_id     BIGINT       NOT NULL,
    resource_id BIGINT       NOT NULL,
    start_date  DATE         NOT NULL,
    end_date    DATE         NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Индекс для фильтрации по пользователю
CREATE INDEX idx_bookings_user_id ON bookings (user_id);

-- Индекс для фильтрации по ресурсу
CREATE INDEX idx_bookings_resource_id ON bookings (resource_id);

-- Индекс для фильтрации по статусу (используется воркером подтверждения)
CREATE INDEX idx_bookings_status ON bookings (status);

-- Составной индекс для пагинации (сортировка по id DESC)
CREATE INDEX idx_bookings_id_desc ON bookings (id DESC);

-- +goose Down
DROP TABLE IF EXISTS bookings;

-- +goose Up
ALTER TABLE bookings 
ADD COLUMN IF NOT EXISTS previous_status VARCHAR(30);
