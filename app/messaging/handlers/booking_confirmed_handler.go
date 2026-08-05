package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"booking-service/app/messaging"
	"booking-service/app/models"
	"booking-service/app/service"
)

// BookingConfirmedHandler обрабатывает события BookingJobConfirmed.
type BookingConfirmedHandler struct {
	service *service.BookingsService
	logger  *zap.Logger
	repo    models.BookingRepository
}

// NewBookingConfirmedHandler создаёт новый обработчик.
func NewBookingConfirmedHandler(svc *service.BookingsService, logger *zap.Logger) *BookingConfirmedHandler {
	return &BookingConfirmedHandler{
		service: svc,
		logger:  logger,
		
	}
}

// Handle обрабатывает событие подтверждения бронирования.
func (h *BookingConfirmedHandler) Handle(ctx context.Context, body []byte) error {
	var event messaging.BookingJobConfirmed
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("десериализация BookingJobConfirmed: %w", err)
	}

	bookingID, err := messaging.RequestIDToBookingID(event.RequestId)
	if err != nil {
		return fmt.Errorf("извлечение bookingId из RequestId: %w", err)
	}

	h.logger.Info("получено событие BookingJobConfirmed",
		zap.Int64("bookingId", bookingID),
		zap.Int64("catalogJobId", event.Id),
	)
	
	raceCondition, err := h.service.Confirm(ctx, bookingID)
	if err != nil {
		if errors.Is(err, models.ErrInvalidStatusTransition) {
			h.logger.Warn("не удалось подтвердить бронирование: недопустимый переход статуса",
				zap.Int64("bookingId", bookingID),
				zap.Error(err),
			)
			return nil 
		}
		return fmt.Errorf("подтверждение бронирования %d: %w", bookingID, err)
	}

	if raceCondition {
		h.logger.Warn("обнаружен race condition: Catalog подтвердил бронирование, пока оно ожидало отмены",
			zap.Int64("bookingId", bookingID),
			zap.Int64("catalogJobId", event.Id),
		)
	}

	h.logger.Info("бронирование подтверждено через событие", zap.Int64("bookingId", bookingID))
	return nil
}

func (h *BookingConfirmedHandler) Handle(ctx context.Context, body []byte) error {
    var event messaging.BookingJobConfirmed
    if err := json.Unmarshal(body, &event); err != nil {
        return fmt.Errorf("десериализация BookingJobConfirmed: %w", err)
    }

    bookingID, err := messaging.RequestIDToBookingID(event.RequestId)
    if err != nil {
        return fmt.Errorf("извлечение bookingId из RequestId: %w", err)
    }

    eventID := fmt.Sprintf("booking_confirmed_%d_%d", bookingID, event.Id)

    h.logger.Info("получено событие BookingJobConfirmed",
        zap.Int64("bookingId", bookingID),
        zap.Int64("catalogJobId", event.Id),
        zap.String("eventId", eventID),
    )

    
    alreadyProcessed, err := h.repo.IsEventProcessed(ctx, eventID)
    if err != nil {
        return fmt.Errorf("проверка идемпотентности: %w", err)
    }

    if alreadyProcessed {
        h.logger.Warn("событие уже обработано (пропускаем)",
            zap.String("eventId", eventID),
            zap.Int64("bookingId", bookingID),
        )
        return nil
    }

    
    raceCondition, err := h.repo.ConfirmWithIdempotency(ctx, bookingID, eventID)
    if err != nil {
        if errors.Is(err, models.ErrInvalidStatusTransition) {
            h.logger.Warn("не удалось подтвердить бронирование: недопустимый переход статуса",
                zap.Int64("bookingId", bookingID),
                zap.Error(err),
            )
            
            h.repo.MarkEventProcessed(ctx, eventID, "BookingConfirmed", bookingID)
            return nil
        }
        return fmt.Errorf("подтверждение бронирования %d: %w", bookingID, err)
    }

    if raceCondition {
        h.logger.Warn("обнаружен race condition: catalog подтвердил бронирование, пока оно ожидало отмены",
            zap.Int64("bookingId", bookingID),
            zap.Int64("catalogJobId", event.Id),
        )
    }

    h.logger.Info("бронирование подтверждено через событие", zap.Int64("bookingId", bookingID))
    return nil
}
