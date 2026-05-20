package stream

import "context"

// Sink 接收会话过程消息；Publish 应阻塞直至消息被消费或 ctx 取消。
type Sink interface {
	Publish(ctx context.Context, msg Message) error
}

// NopSink 丢弃所有消息，供非流式 Run 使用。
type NopSink struct{}

func (NopSink) Publish(context.Context, Message) error { return nil }

// FuncSink 使用函数实现 Sink。
type FuncSink func(ctx context.Context, msg Message) error

func (f FuncSink) Publish(ctx context.Context, msg Message) error {
	if f == nil {
		return nil
	}
	return f(ctx, msg)
}

// ChanSink 将消息写入 channel（阻塞发送）。
type ChanSink struct {
	Ch chan<- Message
}

func (s ChanSink) Publish(ctx context.Context, msg Message) error {
	if s.Ch == nil {
		return nil
	}
	select {
	case s.Ch <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
