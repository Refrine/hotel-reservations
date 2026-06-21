package models

// StatisticsResult содержит агрегированную статистику бронирований за период
type StatisticsResult struct {
	TotalBookings   int64
	StatusBreakdown map[BookingStatus]int64
	TopResources    []ResourceBookingCount
}

// ResourceBookingCount — количество бронирований по ресурсу
type ResourceBookingCount struct {
	ResourceID   int64
	ResourceName string
	BookingCount int64
}

// AllBookingStatuses возвращает все допустимые статусы бронирования
func AllBookingStatuses() []BookingStatus {
	return []BookingStatus{
		BookingStatusAwaitsConfirmation,
		BookingStatusConfirmed,
		BookingStatusCancelled,
		BookingStatusCancellationPending,
	}
}
