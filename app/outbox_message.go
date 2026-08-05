package models

import (
    "encoding/json"
    "time"
)

// OutboxMessageStatus -- статус сообщения в outbox
type OutboxMessageStatus string

const (
    OutboxStatusPending OutboxMessageStatus = "pending"
    OutboxStatusSent    OutboxMessageStatus = "sent"
    OutboxStatusFailed  OutboxMessageStatus = "failed"
)

// OutboxMessage -- сообщение для transactional outbox
type OutboxMessage struct {
    ID          int64
    Exchange    string
    RoutingKey  string
    Payload     json.RawMessage
    ContentType string
    Status      OutboxMessageStatus
    RetryCount  int
    MaxRetries  int
    LastError   *string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}