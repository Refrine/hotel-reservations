package worker

import (
    "context"
    "time"

    amqp "github.com/rabbitmq/amqp091-go"
    "go.uber.org/zap"

    "booking-service/app/models"
)

// OutboxWorker -- воркер для отправки сообщений из outbox
type OutboxWorker struct {
    repo     models.BookingRepository
    channel  *amqp.Channel
    interval time.Duration
    batchSize int
    logger   *zap.Logger
}

// NewOutboxWorker создаёт новый outbox worker
func NewOutboxWorker(
    repo models.BookingRepository,
    channel *amqp.Channel,
    interval time.Duration,
    batchSize int,
    logger *zap.Logger,
) *OutboxWorker {
    return &OutboxWorker{
        repo:     repo,
        channel:  channel,
        interval: interval,
        batchSize: batchSize,
        logger:   logger,
    }
}

// Run запускает воркер
func (w *OutboxWorker) Run(ctx context.Context) {
    w.logger.Info("outbox worker запущен",
        zap.Duration("interval", w.interval),
        zap.Int("batchSize", w.batchSize),
    )

    ticker := time.NewTicker(w.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            w.logger.Info("outbox worker остановлен")
            return
        case <-ticker.C:
            w.processBatch(ctx)
        }
    }
}

// processBatch обрабатывает пакет сообщений
func (w *OutboxWorker) processBatch(ctx context.Context) {
    messages, err := w.repo.GetPendingOutboxMessages(ctx, w.batchSize)
    if err != nil {
        w.logger.Error("ошибка получения сообщений из outbox", zap.Error(err))
        return
    }

    if len(messages) == 0 {
        return
    }

    w.logger.Debug("обработка outbox сообщений", zap.Int("count", len(messages)))

    for _, msg := range messages {
        w.processMessage(ctx, &msg)
    }
}

// processMessage обрабатывает одно сообщение
func (w *OutboxWorker) processMessage(ctx context.Context, msg *models.OutboxMessage) {
    logger := w.logger.With(zap.Int64("messageId", msg.ID))

    err := w.channel.PublishWithContext(
        ctx,
        msg.Exchange,
        msg.RoutingKey,
        false,
        false,
        amqp.Publishing{
            ContentType:  msg.ContentType,
            DeliveryMode: amqp.Persistent,
            Body:         msg.Payload,
            Timestamp:    time.Now(),
        },
    )

    if err != nil {
        logger.Error("ошибка публикации outbox сообщения",
            zap.Error(err),
            zap.Int("retryCount", msg.RetryCount),
        )
        
        if err := w.repo.MarkOutboxFailed(ctx, msg.ID, err.Error()); err != nil {
            logger.Error("ошибка обновления статуса outbox", zap.Error(err))
        }
        return
    }

    if err := w.repo.MarkOutboxSent(ctx, msg.ID); err != nil {
        logger.Error("ошибка отметки сообщения как отправленного", zap.Error(err))
        return
    }

    logger.Debug("outbox сообщение отправлено",
        zap.String("routingKey", msg.RoutingKey),
    )
}