package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"booking-service/app/models"
)

// BookingsRepository реализует models.BookingRepository.
type BookingsRepository struct {
	pool *pgxpool.Pool
}

// NewBookingsRepository создаёт новый экземпляр BookingsRepository.
func NewBookingsRepository(pool *pgxpool.Pool) *BookingsRepository {
	return &BookingsRepository{pool: pool}
}

// Create сохраняет новое бронирование.
func (r *BookingsRepository) Create(ctx context.Context, booking *models.Booking) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, queryInsertBooking,
		string(booking.Status()),
		booking.UserID(),
		booking.ResourceID(),
		booking.StartDate(),
		booking.EndDate(),
		booking.CreatedAt(),
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("создание бронирования: %w", err)
	}
	return id, nil
}

// GetByID возвращает бронирование по ID.
func (r *BookingsRepository) GetByID(ctx context.Context, id int64) (*models.Booking, error) {
	booking, err := r.scanBooking(r.pool.QueryRow(ctx, queryGetBookingByID, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrBookingNotFound
		}
		return nil, fmt.Errorf("получение бронирования id=%d: %w", id, err)
	}
	return booking, nil
}

// Update обновляет статус бронирования.
func (r *BookingsRepository) Update(ctx context.Context, booking *models.Booking) error {
	tag, err := r.pool.Exec(ctx, queryUpdateBookingStatus,
		string(booking.Status()),
		booking.ID(),
	)
	if err != nil {
		return fmt.Errorf("обновление бронирования id=%d: %w", booking.ID(), err)
	}
	if tag.RowsAffected() == 0 {
		return models.ErrBookingNotFound
	}
	return nil
}

// GetByFilter возвращает бронирования с фильтрацией и пагинацией.
func (r *BookingsRepository) GetByFilter(ctx context.Context, filter models.BookingFilter) ([]models.Booking, int64, error) {
	offset := (filter.Page - 1) * filter.Size

	var userID, resourceID *int64
	var status *string
	if filter.UserID != nil {
		userID = filter.UserID
	}
	if filter.ResourceID != nil {
		resourceID = filter.ResourceID
	}
	if filter.Status != nil {
		s := string(*filter.Status)
		status = &s
	}

	// Получение общего количества
	var totalCount int64
	err := r.pool.QueryRow(ctx, queryCountBookingsByFilter, userID, resourceID, status).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("подсчёт бронирований: %w", err)
	}

	// Получение данных
	rows, err := r.pool.Query(ctx, queryGetBookingsByFilter, userID, resourceID, status, filter.Size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("получение бронирований по фильтру: %w", err)
	}
	defer rows.Close()

	var bookings []models.Booking
	for rows.Next() {
		booking, err := r.scanBookingFromRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("сканирование бронирования: %w", err)
		}
		bookings = append(bookings, *booking)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("итерация по строкам: %w", err)
	}

	return bookings, totalCount, nil
}

// GetAwaitingConfirmation возвращает бронирования, ожидающие подтверждения,
// с пессимистичной блокировкой FOR UPDATE SKIP LOCKED.
func (r *BookingsRepository) GetAwaitingConfirmation(ctx context.Context, limit int) ([]models.Booking, error) {
	rows, err := r.pool.Query(ctx, queryGetAwaitingConfirmation, limit)
	if err != nil {
		return nil, fmt.Errorf("получение бронирований для подтверждения: %w", err)
	}
	defer rows.Close()

	var bookings []models.Booking
	for rows.Next() {
		booking, err := r.scanBookingFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("сканирование бронирования: %w", err)
		}
		bookings = append(bookings, *booking)
	}

	return bookings, rows.Err()
}

// scanBooking сканирует одну строку в доменный объект Booking.
func (r *BookingsRepository) scanBooking(row pgx.Row) (*models.Booking, error) {
	var (
		id         int64
		status     string
		userID     int64
		resourceID int64
		startDate  time.Time
		endDate    time.Time
		createdAt  time.Time
		previousStatus      *string    
		cancelCommandSentAt *time.Time 
	)

	err := row.Scan(&id, &status, &userID, &resourceID, &startDate, &endDate, &createdAt, &previousStatus,&cancelCommandSentAt)
	if err != nil {
		return nil, err
	}

	return models.RestoreBooking(id, models.BookingStatus(status), userID, resourceID, startDate, endDate, createdAt, (*models.BookingStatus)(previousStatus), cancelCommandSentAt), nil
}

// scanBookingFromRows сканирует строку из pgx.Rows.
func (r *BookingsRepository) scanBookingFromRows(rows pgx.Rows) (*models.Booking, error) {
	var (
		id         int64
		status     string
		userID     int64
		resourceID int64
		startDate  time.Time
		endDate    time.Time
		createdAt  time.Time
		previousStatus      *string    
		cancelCommandSentAt *time.Time 
		
	)

	err := rows.Scan(&id, &status, &userID, &resourceID, &startDate, &endDate, &createdAt)
	if err != nil {
		return nil, err
	}

	return models.RestoreBooking(id, models.BookingStatus(status), userID, resourceID, startDate, endDate, createdAt, (*models.BookingStatus)(previousStatus), cancelCommandSentAt), nil
}



// GetStatistics получает агрегированную статистику бронирований
func (r *BookingsRepository) GetStatistics(ctx context.Context, dateFrom, dateTo string) (*models.StatisticsResult, error) {
	result := &models.StatisticsResult{
		StatusBreakdown: make(map[models.BookingStatus]int64, len(models.AllBookingStatuses())),
		TopResources:    make([]models.ResourceBookingCount, 0),
	}

	err := r.pool.QueryRow(ctx, queryCountBookingsByCreatedAtRange, dateFrom, dateTo).Scan(&result.TotalBookings)
	if err != nil {
		return nil, fmt.Errorf("подсчёт бронирований: %w", err)
	}

	rows, err := r.pool.Query(ctx, queryStatusBreakdownByCreatedAtRange, dateFrom, dateTo)
	if err != nil {
		return nil, fmt.Errorf("разбивка по статусам: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("сканирование статуса: %w", err)
		}
		result.StatusBreakdown[models.BookingStatus(status)] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("итерация по статусам: %w", err)
	}

	resourceRows, err := r.pool.Query(ctx, queryTopResourcesByCreatedAtRange, dateFrom, dateTo)
	if err != nil {
		return nil, fmt.Errorf("топ ресурсов: %w", err)
	}
	defer resourceRows.Close()

	for resourceRows.Next() {
		var res models.ResourceBookingCount
		if err := resourceRows.Scan(&res.ResourceID, &res.BookingCount); err != nil {
			return nil, fmt.Errorf("сканирование ресурса: %w", err)
		}
		result.TopResources = append(result.TopResources, res)
	}
	if err := resourceRows.Err(); err != nil {
		return nil, fmt.Errorf("итерация по ресурсам: %w", err)
	}

	return result, nil
}


func (r *BookingsRepository) GetCancellationPendingOlderThan(ctx context.Context, olderThan time.Duration, limit int) ([]models.Booking, error) {
	cutoff := time.Now().Add(-olderThan)

	// Запрос должен возвращать те же столбцы, что и scanBookingFromRows ожидает.
	query := `
	SELECT id, status, user_id, resource_id, start_date, end_date, created_at
	FROM bookings
	WHERE status = 'cancellation_pending' AND updated_at <= $1
	ORDER BY updated_at ASC
	LIMIT $2
	FOR UPDATE SKIP LOCKED
	`

	// Открываем транзакцию, чтобы использовать FOR UPDATE SKIP LOCKED корректно.
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("начало транзакции: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, query, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("запрос зависших отмен: %w", err)
	}
	defer rows.Close()

	var res []models.Booking
	for rows.Next() {
		booking, err := r.scanBookingFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("сканирование бронирования: %w", err)
		}
		res = append(res, *booking)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("итерация по строкам: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit транзакции: %w", err)
	}

	return res, nil
}

