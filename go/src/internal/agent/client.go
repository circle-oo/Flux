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

func (c *Client) ExecuteTask(
	ctx context.Context,
	req *fluxv1.ExecuteTaskRequest,
	onEvent func(*fluxv1.TaskEvent),
) error {
	stream, err := c.agent.ExecuteTask(ctx, req)
	if err != nil {
		return err
	}
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		event := resp.GetEvent()
		if event == nil {
			continue
		}
		c.logger.Info("agent event",
			"task_id", event.GetTaskId(),
			"type", event.GetType().String(),
		)
		onEvent(event)
		if event.GetType() == fluxv1.TaskEvent_TASK_EVENT_TYPE_TASK_COMPLETE ||
			event.GetType() == fluxv1.TaskEvent_TASK_EVENT_TYPE_TASK_ERROR {
			return nil
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
