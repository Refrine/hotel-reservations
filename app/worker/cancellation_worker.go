package worker

import (
	"context"
	"time"

	"go.uber.org/zap"

	"booking-service/app/messaging"
	"booking-service/app/models"
	
)

type CancellationWorker struct {
	repo      models.BookingRepository
	publisher *messaging.Publisher
	interval  time.Duration
	batchSize int
	timeout   time.Duration
	logger    *zap.Logger
}

func NewCancellationWorker(
	repo models.BookingRepository,
	publisher *messaging.Publisher,
	interval time.Duration,
	batchSize int,
	timeout time.Duration,
	logger *zap.Logger,
) *CancellationWorker {
	return &CancellationWorker{
		repo:      repo,
		publisher: publisher,
		interval:  interval,
		batchSize: batchSize,
		timeout:   timeout,
		logger:    logger,
	}
}

func (w *CancellationWorker) Run(ctx context.Context) {
	w.logger.Info("воркер отмены запущен",
		zap.Duration("interval", w.interval),
		zap.Int("batchSize", w.batchSize),
		zap.Duration("timeout", w.timeout),
	)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("воркер отмены остановлен")
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *CancellationWorker) processBatch(ctx context.Context) {
	bookings, err := w.repo.GetCancellationPendingOlderThan(ctx, w.timeout, w.batchSize)
	if err != nil {
		w.logger.Error("ошибка получения зависших отмен", zap.Error(err))
		return
	}

	if len(bookings) == 0 {
		return
	}

	w.logger.Info("обработка зависших отмен", zap.Int("count", len(bookings)))

	for _, b := range bookings {
		
		func(booking models.Booking) {
			bookingID := booking.ID()
			logger := w.logger.With(zap.Int64("bookingId", bookingID))

			if err := w.publisher.PublishCancelBookingJob(ctx, bookingID); err != nil {
				logger.Error("ошибка публикации задания отмены в Catalog", zap.Error(err))
				
				return
			}

			logger.Info("повторная публикация задания отмены отправлена")
		}(b)
	}
}
