package queue

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

const (
	ExchangeName       = "dockrunner"
	QueueDeploys       = "deploys"
	QueueNotifications = "notifications"
	QueueEvents        = "events"
)

type Config struct {
	URL           string
	TLSSkipVerify bool
}

func dialURL(cfg Config) (*amqp.Connection, error) {
	var conn *amqp.Connection
	var err error
	if strings.HasPrefix(cfg.URL, "amqps://") && cfg.TLSSkipVerify {
		conn, err = amqp.DialTLS(cfg.URL, &tls.Config{InsecureSkipVerify: true})
	} else {
		conn, err = amqp.Dial(cfg.URL)
	}
	if err != nil && errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w (hint: amqp:// to a TLS port or amqps:// to a plain port drops handshake with EOF — check scheme and port)", err)
	}
	return conn, err
}

type Client struct {
	conn   *amqp.Connection
	ch     *amqp.Channel
	logger *zap.Logger
}

func Dial(ctx context.Context, cfg Config, logger *zap.Logger) (*Client, error) {
	var conn *amqp.Connection
	var err error
	for attempt := 1; attempt <= 5; attempt++ {
		conn, err = dialURL(cfg)
		if err == nil {
			break
		}
		logger.Warn("rabbitmq dial failed, retrying",
			zap.Int("attempt", attempt), zap.Error(err))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial failed: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq open channel: %w", err)
	}
	if err := ch.Qos(1, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq qos: %w", err)
	}

	logger.Info("rabbitmq connected")
	return &Client{conn: conn, ch: ch, logger: logger}, nil
}

func (c *Client) DeclareTopology(ctx context.Context) error {
	if err := c.ch.ExchangeDeclare(ExchangeName, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}
	for _, q := range []string{QueueDeploys, QueueNotifications, QueueEvents} {
		if _, err := c.ch.QueueDeclare(q, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare queue %s: %w", q, err)
		}
		if err := c.ch.QueueBind(q, q, ExchangeName, false, nil); err != nil {
			return fmt.Errorf("bind queue %s: %w", q, err)
		}
	}
	return nil
}

func (c *Client) Publish(ctx context.Context, routingKey string, body []byte) error {
	return c.ch.PublishWithContext(ctx, ExchangeName, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
	})
}

func (c *Client) RpcCall(ctx context.Context, routingKey string, body []byte, timeout time.Duration) ([]byte, error) {
	replyQueue, err := c.ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return nil, fmt.Errorf("declare reply queue: %w", err)
	}
	msgs, err := c.ch.Consume(replyQueue.Name, "", true, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("consume replies: %w", err)
	}
	corrID := uuid.NewString()
	if err := c.ch.PublishWithContext(ctx, ExchangeName, routingKey, false, false, amqp.Publishing{
		ContentType:   "application/json",
		Body:          body,
		DeliveryMode:  amqp.Persistent,
		Timestamp:     time.Now(),
		ReplyTo:       replyQueue.Name,
		CorrelationId: corrID,
	}); err != nil {
		return nil, err
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("agent timeout")
		case msg, ok := <-msgs:
			if !ok {
				return nil, fmt.Errorf("reply channel closed")
			}
			if msg.CorrelationId == corrID {
				return msg.Body, nil
			}
		}
	}
}

func (c *Client) Consume(queue string, handler func(ctx context.Context, body []byte) error) error {
	msgs, err := c.ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume %s: %w", queue, err)
	}
	go func() {
		for msg := range msgs {
			if err := handler(context.Background(), msg.Body); err != nil {
				if msg.Redelivered {
					c.logger.Error("queue event failed twice, dropping", zap.String("queue", queue), zap.Error(err))
					_ = msg.Ack(false)
					continue
				}
				c.logger.Error("queue handler failed", zap.String("queue", queue), zap.Error(err))
				_ = msg.Nack(false, true)
				continue
			}
			_ = msg.Ack(false)
		}
	}()
	return nil
}

func (c *Client) Close() error {
	if err := c.ch.Close(); err != nil {
		_ = c.conn.Close()
		return err
	}
	return c.conn.Close()
}
