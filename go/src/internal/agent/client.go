package agent

import (
	"context"
	"io"
	"log/slog"
	"time"

	fluxv1 "github.com/circle-oo/flux/gen/flux/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ExecutionResult holds the final output from an agent execution.
type ExecutionResult struct {
	// Result is the final result text from the TASK_COMPLETE event (ResultMessage).
	Result string
	// Output collects all ASSISTANT_MESSAGE content (intermediate conversation).
	Output string
	// CostUSD is extracted from ResultMessage metadata if available.
	CostUSD string
	// NumTurns is extracted from ResultMessage metadata if available.
	NumTurns string
	// SessionID is extracted from ResultMessage metadata if available.
	SessionID string
	// IsError is true if the agent returned a TASK_ERROR.
	IsError bool
	// ErrorMessage holds the error content if IsError is true.
	ErrorMessage string
}

type Client struct {
	conn   *grpc.ClientConn
	agent  fluxv1.AgentExecutionServiceClient
	logger *slog.Logger
}

func NewClient(addr string, logger *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10<<20)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}),
	)
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:   conn,
		agent:  fluxv1.NewAgentExecutionServiceClient(conn),
		logger: logger,
	}, nil
}

// ExecuteTask streams events from the agent and returns a structured result.
// The onEvent callback is called for every event (for logging/streaming to UI).
// The returned ExecutionResult separates the final result from intermediate output.
func (c *Client) ExecuteTask(
	ctx context.Context,
	req *fluxv1.ExecuteTaskRequest,
	onEvent func(*fluxv1.TaskEvent),
) (*ExecutionResult, error) {
	stream, err := c.agent.ExecuteTask(ctx, req)
	if err != nil {
		return nil, err
	}

	result := &ExecutionResult{}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return result, nil
		}
		if err != nil {
			return result, err
		}
		event := resp.GetEvent()
		if event == nil {
			continue
		}

		c.logger.Debug("agent event",
			"task_id", event.GetTaskId(),
			"type", event.GetType().String(),
		)

		if onEvent != nil {
			onEvent(event)
		}

		switch event.GetType() {
		case fluxv1.TaskEvent_TASK_EVENT_TYPE_ASSISTANT_MESSAGE:
			if event.GetContent() != "" {
				result.Output += event.GetContent() + "\n"
			}

		case fluxv1.TaskEvent_TASK_EVENT_TYPE_TASK_COMPLETE:
			result.Result = event.GetContent()
			// Extract metadata from ResultMessage
			if meta := event.GetMetadata(); meta != nil {
				result.CostUSD = meta["cost_usd"]
				result.NumTurns = meta["num_turns"]
				result.SessionID = meta["session_id"]
			}
			return result, nil

		case fluxv1.TaskEvent_TASK_EVENT_TYPE_TASK_ERROR:
			result.IsError = true
			result.ErrorMessage = event.GetContent()
			if meta := event.GetMetadata(); meta != nil {
				result.CostUSD = meta["cost_usd"]
				result.NumTurns = meta["num_turns"]
			}
			return result, nil
		}
	}
}

func (c *Client) CancelTask(ctx context.Context, taskID string) error {
	_, err := c.agent.CancelAgentTask(ctx,
		&fluxv1.CancelAgentTaskRequest{TaskId: taskID})
	return err
}

func (c *Client) PodStatus(ctx context.Context) (*fluxv1.GetPodStatusResponse, error) {
	return c.agent.GetPodStatus(ctx, &fluxv1.GetPodStatusRequest{})
}

func (c *Client) Close() error {
	return c.conn.Close()
}
