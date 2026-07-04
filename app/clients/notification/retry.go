package notification

import (
    "context"
    "math"
    "time"
)

// retryError -- ошибка с флагом permanent
type retryError struct {
    permanent bool
    message   string
}

func (e *retryError) Error() string {
    return e.message
}

// withRetry выполняет операцию с экспоненциальным backoff
func withRetry(ctx context.Context, maxRetries int, baseDelay time.Duration, operation func() error) error {
    var lastErr error

    for attempt := 0; attempt <= maxRetries; attempt++ {
        if attempt > 0 {
            delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay

            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(delay):
            }
        }

        err := operation()
        if err == nil {
            return nil
        }

        
        var retryErr *retryError
        if e, ok := err.(*retryError); ok && e.permanent {
            return err
        }

        lastErr = err
    }

    return lastErr
}