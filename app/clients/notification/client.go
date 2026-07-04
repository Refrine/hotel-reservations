package notification

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "go.uber.org/zap"
)

// Client -- HTTP клиент для Notification Service
type Client struct {
    baseURL    string
    httpClient *http.Client
    maxRetries int
    baseDelay  time.Duration
    logger     *zap.Logger
}

// SendNotificationRequest -- запрос на отправку уведомления
type SendNotificationRequest struct {
    UserID    int64  `json:"user_id"`
    BookingID int64  `json:"booking_id"`
    Type      string `json:"type"`
    Message   string `json:"message"`
}

// NewClient создаёт новый клиент
func NewClient(baseURL string, timeout time.Duration, maxRetries int, baseDelay time.Duration, logger *zap.Logger) *Client {
    return &Client{
        baseURL: baseURL,
        httpClient: &http.Client{
            Timeout: timeout,
        },
        maxRetries: maxRetries,
        baseDelay:  baseDelay,
        logger:     logger,
    }
}

// SendNotification отправляет уведомление с retry
func (c *Client) SendNotification(ctx context.Context, req SendNotificationRequest) error {
    body, err := json.Marshal(req)
    if err != nil {
        return fmt.Errorf("marshal request: %w", err)
    }

    err = withRetry(ctx, c.maxRetries, c.baseDelay, func() error {
        httpReq, err := http.NewRequestWithContext(
            ctx,
            http.MethodPost,
            c.baseURL+"/api/notifications",
            bytes.NewReader(body),
        )
        if err != nil {
            return fmt.Errorf("create request: %w", err)
        }
        httpReq.Header.Set("Content-Type", "application/json")

        resp, err := c.httpClient.Do(httpReq)
        if err != nil {
            return fmt.Errorf("http request: %w", err)
        }
        defer resp.Body.Close()

        
        if resp.StatusCode >= 400 && resp.StatusCode < 500 {
            return &retryError{
                permanent: true,
                message:   fmt.Sprintf("client error: %d", resp.StatusCode),
            }
        }

       
        if resp.StatusCode >= 500 {
            return fmt.Errorf("server error: %d", resp.StatusCode)
        }

        return nil
    })

    if err != nil {
        c.logger.Warn("не удалось отправить уведомление",
            zap.Int64("userId", req.UserID),
            zap.Int64("bookingId", req.BookingID),
            zap.String("type", req.Type),
            zap.Error(err),
        )
        return nil 
    }

    c.logger.Info("уведомление отправлено",
        zap.Int64("userId", req.UserID),
        zap.Int64("bookingId", req.BookingID),
        zap.String("type", req.Type),
    )

    return nil
}