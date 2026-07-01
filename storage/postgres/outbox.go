package postgres

import (
    "context"
    "fmt"
    "booking-service/app/models"
)


func (r *BookingsRepository) SaveOutboxMessage(ctx context.Context, msg *models.OutboxMessage) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO outbox_messages (exchange, routing_key, payload, content_type, status, max_retries)
        VALUES ($1, $2, $3, $4, $5, $6)
    `, msg.Exchange, msg.RoutingKey, msg.Payload, msg.ContentType, msg.Status, msg.MaxRetries)
    
    return err
}


func (r *BookingsRepository) GetPendingOutboxMessages(ctx context.Context, limit int) ([]models.OutboxMessage, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT id, exchange, routing_key, payload, content_type, status, retry_count, max_retries, last_error, created_at, updated_at
        FROM outbox_messages
        WHERE status = $1
          AND retry_count < max_retries
        ORDER BY created_at ASC
        LIMIT $2
        FOR UPDATE SKIP LOCKED
    `, models.OutboxStatusPending, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var messages []models.OutboxMessage
    for rows.Next() {
        var msg models.OutboxMessage
        if err := rows.Scan(&msg.ID, &msg.Exchange, &msg.RoutingKey, &msg.Payload,
            &msg.ContentType, &msg.Status, &msg.RetryCount, &msg.MaxRetries,
            &msg.LastError, &msg.CreatedAt, &msg.UpdatedAt); err != nil {
            return nil, err
        }
        messages = append(messages, msg)
    }
    
    return messages, rows.Err()
}


func (r *BookingsRepository) MarkOutboxSent(ctx context.Context, id int64) error {
    _, err := r.pool.Exec(ctx, `
        UPDATE outbox_messages 
        SET status = $1, updated_at = NOW() 
        WHERE id = $2
    `, models.OutboxStatusSent, id)
    return err
}


func (r *BookingsRepository) MarkOutboxFailed(ctx context.Context, id int64, errMsg string) error {
    _, err := r.pool.Exec(ctx, `
        UPDATE outbox_messages 
        SET retry_count = retry_count + 1,
            last_error = $1,
            status = CASE WHEN retry_count + 1 >= max_retries THEN $2 ELSE $3 END,
            updated_at = NOW()
        WHERE id = $4
    `, errMsg, models.OutboxStatusFailed, models.OutboxStatusPending, id)
    return err
}


func (r *BookingsRepository) UpdateWithOutbox(ctx context.Context, bookingID int64, updateFn func(*models.Booking) error, outboxMsg *models.OutboxMessage) error {
    tx, err := r.pool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback(ctx)

    var booking models.Booking
    err = tx.QueryRow(ctx, `
        SELECT id, user_id, resource_id, status, date_from, date_to, created_at, updated_at
        FROM bookings WHERE id = $1 FOR UPDATE
    `, bookingID).Scan(&booking.ID, &booking.UserID, &booking.ResourceID,
        &booking.Status, &booking.DateFrom, &booking.DateTo,
        &booking.CreatedAt, &booking.UpdatedAt)
    if err != nil {
        return fmt.Errorf("get booking: %w", err)
    }

   
    if err := updateFn(&booking); err != nil {
        return err
    }

    
    _, err = tx.Exec(ctx, `
        UPDATE bookings SET status = $1, updated_at = NOW() WHERE id = $2
    `, booking.Status, bookingID)
    if err != nil {
        return fmt.Errorf("update booking: %w", err)
    }

   
    if outboxMsg != nil {
        _, err = tx.Exec(ctx, `
            INSERT INTO outbox_messages (exchange, routing_key, payload, content_type, max_retries)
            VALUES ($1, $2, $3, $4, $5)
        `, outboxMsg.Exchange, outboxMsg.RoutingKey, outboxMsg.Payload, 
           outboxMsg.ContentType, outboxMsg.MaxRetries)
        if err != nil {
            return fmt.Errorf("insert outbox: %w", err)
        }
    }

    return tx.Commit(ctx)
}