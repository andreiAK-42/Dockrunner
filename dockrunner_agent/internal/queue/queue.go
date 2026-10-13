package queue

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"log/slog"
)

const (
	Exchange    = "dockrunner"
	QueueEvents = "events"
)

func AgentQueue(agentID string) string {
	return "agent." + agentID
}

type Client struct {
	conn   *amqp.Connection
	ch     *amqp.Channel
	logger *slog.Logger
}

// PermanentError marks a task failure that no retry will fix
// (bad payload, missing local state). Such messages are acked, not requeued.
type PermanentError struct {
	Err error
}

func (e PermanentError) Error() string { return e.Err.Error() }
func (e PermanentError) Unwrap() error { return e.Err }

func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return PermanentError{Err: err}
}

func IsPermanent(err error) bool {
	var perm PermanentError
	return errors.As(err, &perm)
}

func dialURL(url string, tlsSkipVerify bool) (*amqp.Connection, error) {
	var conn *amqp.Connection
	var err error
	if strings.HasPrefix(url, "amqps://") && tlsSkipVerify {
		conn, err = amqp.DialTLS(url, &tls.Config{InsecureSkipVerify: true})
	} else {
		conn, err = amqp.Dial(url)
	}
	if err != nil && errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w (hint: amqp:// to a TLS port or amqps:// to a plain port drops handshake with EOF — check scheme and port)", err)
	}
	return conn, err
}

func Dial(ctx context.Context, url string, tlsSkipVerify bool, logger *slog.Logger) (*Client, error) {
	var conn *amqp.Connection
	var err error
	for attempt := 1; attempt <= 5; attempt++ {
		conn, err = dialURL(url, tlsSkipVerify)
		if err == nil {
			break
		}
		logger.Warn("rabbitmq dial failed, retrying", "attempt", attempt, "error", err)
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

func (c *Client) DeclareTopology(agentID string) error {
	if err := c.ch.ExchangeDeclare(Exchange, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}
	tasks := AgentQueue(agentID)
	if _, err := c.ch.QueueDeclare(tasks, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare tasks queue: %w", err)
	}
	if err := c.ch.QueueBind(tasks, tasks, Exchange, false, nil); err != nil {
		return fmt.Errorf("bind tasks queue: %w", err)
	}
	for _, q := range []string{QueueEvents} {
		if _, err := c.ch.QueueDeclare(q, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare queue %s: %w", q, err)
		}
		if err := c.ch.QueueBind(q, q, Exchange, false, nil); err != nil {
			return fmt.Errorf("bind queue %s: %w", q, err)
		}
	}
	return nil
}

func (c *Client) Publish(ctx context.Context, routingKey string, body []byte) error {
	return c.ch.PublishWithContext(ctx, Exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
	})
}

func (c *Client) Consume(queue string, parallel int, handler func(ctx context.Context, msg amqp.Delivery) error) error {
	if parallel < 1 {
		parallel = 1
	}
	if err := c.ch.Qos(parallel, 0, false); err != nil {
		return fmt.Errorf("qos %s: %w", queue, err)
	}
	msgs, err := c.ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume %s: %w", queue, err)
	}
	sem := make(chan struct{}, parallel)
	go func() {
		for msg := range msgs {
			sem <- struct{}{}
			go func(m amqp.Delivery) {
				defer func() { <-sem }()
				defer func() {
					if r := recover(); r != nil {
						if m.Redelivered {
							c.logger.Error("queue handler panic twice, dropping", "queue", queue, "panic", r)
							_ = m.Ack(false)
							return
						}
						c.logger.Error("queue handler panic", "queue", queue, "panic", r)
						_ = m.Nack(false, true)
					}
				}()
				if err := handler(context.Background(), m); err != nil {
					if IsPermanent(err) {
						c.logger.Warn("queue task permanent failure, dropping", "queue", queue, "error", err)
						_ = m.Ack(false)
						return
					}
					if m.Redelivered {
						c.logger.Error("queue task failed twice, dropping", "queue", queue, "error", err)
						_ = m.Ack(false)
						return
					}
					c.logger.Error("queue handler failed", "queue", queue, "error", err)
					_ = m.Nack(false, true)
					return
				}
				_ = m.Ack(false)
			}(msg)
		}
	}()
	return nil
}

func (c *Client) Reply(ctx context.Context, msg amqp.Delivery, body []byte) error {
	if msg.ReplyTo == "" {
		return nil
	}
	return c.ch.PublishWithContext(ctx, "", msg.ReplyTo, false, false, amqp.Publishing{
		ContentType:   "application/json",
		Body:          body,
		CorrelationId: msg.CorrelationId,
		Timestamp:     time.Now(),
	})
}

func (c *Client) Close() error {
	if err := c.ch.Close(); err != nil {
		_ = c.conn.Close()
		return err
	}
	return c.conn.Close()
}
