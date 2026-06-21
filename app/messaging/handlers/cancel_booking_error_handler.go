package handlers

import (
	"context"
	"encoding/json"
	"log"
)


type BookingCancelService interface {
	HandleCancelError(ctx context.Context, requestID string) error
}


type CancelBookingErrorHandler struct {
	service BookingCancelService
}


type CancelBookingError struct {
	RequestID string `json:"request_id"`
	Error     string `json:"error"`
}

func NewCancelBookingErrorHandler(service BookingCancelService) *CancelBookingErrorHandler {
	return &CancelBookingErrorHandler{service: service}
}


func (h *CancelBookingErrorHandler) Handle(ctx context.Context, message []byte) error {
	var cancelErr CancelBookingError
	if err := json.Unmarshal(message, &cancelErr); err != nil {
		log.Printf("fail to unmarshal: %v", err)
		return err
	}

	log.Printf("processing DLQ %s: %s", cancelErr.RequestID, cancelErr.Error)
	
	
	return h.service.HandleCancelError(ctx, cancelErr.RequestID)
}