package agents

import (
	"bufio"
	"context"
	"io"
	"sync/atomic"
)

type inputTarget int32

const (
	inputTargetChat inputTarget = iota
	inputTargetApproval
)

// InputRouter multiplexes stdin between the chat loop and approval prompts.
// Only one goroutine reads the underlying reader so input is never split across consumers.
// InputRouter 在聊天循环与审批提示之间复用 stdin。
// 仅由一个 goroutine 读取底层 reader，避免输入被多个消费者争抢。
type InputRouter struct {
	target     atomic.Int32
	chatCh     chan promptResult
	approvalCh chan string
}

// NewInputRouter starts the stdin reader goroutine.
// NewInputRouter 启动 stdin 读取 goroutine。
func NewInputRouter(ctx context.Context, reader io.Reader) *InputRouter {
	router := &InputRouter{
		chatCh:     make(chan promptResult),
		approvalCh: make(chan string, 4),
	}
	router.target.Store(int32(inputTargetChat))
	go router.readLoop(ctx, reader)
	return router
}

// ChatPrompts returns the channel used by the interactive chat loop.
// ChatPrompts 返回交互式聊天循环使用的输入通道。
func (r *InputRouter) ChatPrompts() <-chan promptResult {
	if r == nil {
		return nil
	}
	return r.chatCh
}

// BeginApproval routes subsequent stdin lines to the approval prompter.
// BeginApproval 将后续 stdin 行路由到审批提示器。
func (r *InputRouter) BeginApproval() {
	if r == nil {
		return
	}
	r.target.Store(int32(inputTargetApproval))
}

// EndApproval returns routing to the chat loop and discards stale approval input.
// EndApproval 将路由切回聊天循环并丢弃残留的审批输入。
func (r *InputRouter) EndApproval() {
	if r == nil {
		return
	}
	r.target.Store(int32(inputTargetChat))
	r.drainApproval()
}

// ReadApprovalLine waits for one approval response line.
// ReadApprovalLine 等待一行审批响应。
func (r *InputRouter) ReadApprovalLine(ctx context.Context) (string, error) {
	if r == nil {
		return "", io.EOF
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case line := <-r.approvalCh:
		return line, nil
	}
}

func (r *InputRouter) readLoop(ctx context.Context, reader io.Reader) {
	defer close(r.chatCh)

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if r.target.Load() == int32(inputTargetApproval) {
			select {
			case r.approvalCh <- line:
			case <-ctx.Done():
				return
			}
			continue
		}
		select {
		case r.chatCh <- promptResult{text: line}:
		case <-ctx.Done():
			return
		}
	}
	if err := scanner.Err(); err != nil {
		select {
		case r.chatCh <- promptResult{err: err}:
		case <-ctx.Done():
		}
	}
}

func (r *InputRouter) drainApproval() {
	for {
		select {
		case <-r.approvalCh:
		default:
			return
		}
	}
}
