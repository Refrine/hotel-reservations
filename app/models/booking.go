package models

import "time"

// BookingStatus представляет статус бронирования.
type BookingStatus string

const (
	BookingStatusAwaitsConfirmation BookingStatus = "awaits_confirmation"
	BookingStatusConfirmed          BookingStatus = "confirmed"
	BookingStatusCancelled          BookingStatus = "cancelled"
	BookingStatusCancellationPending BookingStatus = "cancellation_pending"
	
)


// IsValid проверяет, что статус принадлежит допустимому множеству.
func (s BookingStatus) IsValid() bool {
	switch s {
	case BookingStatusAwaitsConfirmation, BookingStatusConfirmed, BookingStatusCancelled, BookingStatusCancellationPending:
		return true
	default:
		return false
	}
}

// Booking -- доменная сущность бронирования.
// Поля неэкспортируемые для обеспечения инкапсуляции.
type Booking struct {
	id         int64
	status     BookingStatus
	userID     int64
	resourceID int64
	startDate  time.Time
	endDate    time.Time
	createdAt  time.Time
	updatedAt time.Time
	previousStatus *BookingStatus
	cancelCommandSentAt *time.Time 
	CancellationRequestedAt *time.Time
	CancellationReason      *string
}

func (b *Booking) ID() int64             { return b.id }
func (b *Booking) Status() BookingStatus { return b.status }
func (b *Booking) UserID() int64         { return b.userID }
func (b *Booking) ResourceID() int64     { return b.resourceID }
func (b *Booking) StartDate() time.Time  { return b.startDate }
func (b *Booking) EndDate() time.Time    { return b.endDate }
func (b *Booking) CreatedAt() time.Time  { return b.createdAt }
func (b *Booking) PreviousStatus() *BookingStatus {return  b.previousStatus}
func (b *Booking) CancelCommandSentAt() *time.Time {return b.cancelCommandSentAt}

// NewBooking создаёт новое бронирование в статусе AwaitsConfirmation.
func NewBooking(userID, resourceID int64, startDate, endDate time.Time) (*Booking, error) {
	if userID <= 0 {
		return nil, ErrInvalidUserID
	}
	if resourceID <= 0 {
		return nil, ErrInvalidResourceID
	}
	if startDate.IsZero() || endDate.IsZero() {
		return nil, ErrInvalidDateRange
	}
	if !endDate.After(startDate) {
		return nil, ErrEndDateBeforeStartDate
	}

	return &Booking{
		status:     BookingStatusAwaitsConfirmation,
		userID:     userID,
		resourceID: resourceID,
		startDate:  startDate,
		endDate:    endDate,
		createdAt:  time.Now(),
	}, nil
}

// Confirm подтверждает бронирование.
// Допустимые переходы:
//   - AwaitsConfirmation -> Confirmed
//   - CancellationPending -> Confirmed (race condition: Catalog подтвердил, пока ожидалась отмена)
func (b *Booking) Confirm() error {
	if b.status != BookingStatusAwaitsConfirmation && b.status != BookingStatusCancellationPending {
		return ErrInvalidStatusTransition
	}

	if b.status == BookingStatusCancellationPending {
		b.previousStatus = nil
		b.cancelCommandSentAt = nil
		b.CancellationRequestedAt = nil
		b.CancellationReason = nil
	}

	b.status = BookingStatusConfirmed
	b.updatedAt = time.Now()
	return nil
}

// Cancel отменяет бронирование.
// Допустимые переходы:
//   - AwaitsConfirmation -> Cancelled
//   - Confirmed -> Cancelled (только если StartDate > today)
func (b *Booking) Cancel(today time.Time) error {
	switch b.status {
	case BookingStatusAwaitsConfirmation:
		b.status = BookingStatusCancelled
		return nil
	case BookingStatusConfirmed:
		if !b.startDate.After(today) {
			return ErrCannotCancelPastBooking
		}
		b.status = BookingStatusCancelled
		return nil
	case BookingStatusCancelled:
		return ErrInvalidStatusTransition
	default:
		return ErrInvalidStatusTransition
	}
}


func (b *Booking) StartCancellation(today time.Time) error {
	switch b.status {
	case BookingStatusAwaitsConfirmation:
		
	case BookingStatusConfirmed:
		if !b.startDate.After(today) {
			return ErrCannotCancelPastBooking
		}
	case BookingStatusCancellationPending:
		return nil 
	default:
		return ErrInvalidStatusTransition
	}

	prevStatus := b.status
	b.previousStatus = &prevStatus
	b.status = BookingStatusCancellationPending
	b.cancelCommandSentAt = &[]time.Time{time.Now()}[0]
	return nil
}


func (b *Booking) CompleteCancellation() error {
	if b.status != BookingStatusCancellationPending {
		return ErrInvalidStatusTransition
	}
	b.status = BookingStatusCancelled
	b.previousStatus = nil
	b.cancelCommandSentAt = nil
	return nil
}

func (b *Booking) RollbackCancellation() error {
	if b.status != BookingStatusCancellationPending {
		return ErrInvalidStatusTransition
	}
	if b.previousStatus == nil {
		return ErrInvalidStatusTransition
	}
	b.status = *b.previousStatus
	b.previousStatus = nil
	b.cancelCommandSentAt = nil
	return nil
}

// RestoreBooking восстанавливает Booking из данных хранилища.
// Используется только в слое storage для маппинга строк БД на доменный объект.
func RestoreBooking(
	id int64,
	status BookingStatus,
	userID, resourceID int64,
	startDate, endDate, createdAt time.Time,
	previousStatus *BookingStatus,    
	cancelCommandSentAt *time.Time,   
) *Booking {
	return &Booking{
		id:         id,
		status:     status,
		userID:     userID,
		resourceID: resourceID,
		startDate:  startDate,
		endDate:    endDate,
		createdAt:  createdAt,
		previousStatus:	previousStatus,      
		cancelCommandSentAt:	cancelCommandSentAt, 
	}
}
