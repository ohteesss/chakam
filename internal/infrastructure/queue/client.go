package queue

import (
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"chakam/internal/config"

	"github.com/hibiken/asynq"
)

type Client struct {
	inner *asynq.Client
}

func NewClient(cfg config.RedisConfig) *Client {
	return &Client{
		inner: asynq.NewClient(RedisClientOpt(cfg)),
	}
}

func RedisClientOpt(cfg config.RedisConfig) asynq.RedisClientOpt {
	opt := asynq.RedisClientOpt{
		Addr:         fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	if cfg.TLS {
		opt.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return opt
}

func (c *Client) EnqueueProcessMessage(payload ProcessMessagePayload) error {
	task, err := NewProcessMessageTask(payload)
	if err != nil {
		return err
	}
	options := []asynq.Option{asynq.MaxRetry(2), asynq.Queue("critical"), asynq.Retention(24 * time.Hour)}
	if payload.MessageID != nil && *payload.MessageID != "" {
		options = append(options, asynq.TaskID(*payload.MessageID))
	}
	_, err = c.inner.Enqueue(task, options...)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}

func (c *Client) EnqueueSendMessage(payload SendMessagePayload) error {
	task, err := NewSendMessageTask(payload.To, payload.Message)
	if err != nil {
		return err
	}
	_, err = c.inner.Enqueue(task, asynq.MaxRetry(2), asynq.Queue("default"))
	return err
}

func (c *Client) Ping() error {
	return c.inner.Ping()
}

func (c *Client) Close() error {
	return c.inner.Close()
}
