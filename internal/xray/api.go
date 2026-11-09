package xray

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	command "github.com/xtls/xray-core/app/proxyman/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	hyaccount "github.com/xtls/xray-core/proxy/hysteria/account"
	"github.com/xtls/xray-core/proxy/vless"
)

// UserAPI changes users of running inbounds without restarting xray.
type UserAPI interface {
	Ping(ctx context.Context) error
	AddUser(ctx context.Context, tag string, kind Kind, u User) error
	RemoveUser(ctx context.Context, tag, email string) error
	Users(ctx context.Context, tag string) ([]string, error)
}

// Client talks to xray's HandlerService on 127.0.0.1:<api_port>.
type Client struct {
	conn *grpc.ClientConn
	h    command.HandlerServiceClient
}

func Dial(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("xray api %s: %w", addr, err)
	}
	return &Client{conn: conn, h: command.NewHandlerServiceClient(conn)}, nil
}

func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.h.ListInbounds(ctx, &command.ListInboundsRequest{IsOnlyTags: true})
	if err != nil {
		return fmt.Errorf("xray api: %w", err)
	}
	return nil
}

func account(kind Kind, u User) (*serial.TypedMessage, error) {
	switch kind {
	case KindVision:
		return serial.ToTypedMessage(&vless.Account{Id: u.VlessUUID, Flow: FlowVision, Encryption: "none"}), nil
	case KindXHTTP:
		return serial.ToTypedMessage(&vless.Account{Id: u.VlessUUID, Encryption: "none"}), nil
	case KindHysteria2:
		return serial.ToTypedMessage(&hyaccount.Account{Auth: u.Hy2Password}), nil
	}
	return nil, fmt.Errorf("unknown kind %q", kind)
}

func (c *Client) AddUser(ctx context.Context, tag string, kind Kind, u User) error {
	acc, err := account(kind, u)
	if err != nil {
		return err
	}
	op := &command.AddUserOperation{User: &protocol.User{Email: u.Email(), Account: acc}}
	_, err = c.h.AlterInbound(ctx, &command.AlterInboundRequest{Tag: tag, Operation: serial.ToTypedMessage(op)})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("add user to %s: %w", tag, err)
	}
	return nil
}

func (c *Client) RemoveUser(ctx context.Context, tag, email string) error {
	op := &command.RemoveUserOperation{Email: email}
	_, err := c.h.AlterInbound(ctx, &command.AlterInboundRequest{Tag: tag, Operation: serial.ToTypedMessage(op)})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("remove user from %s: %w", tag, err)
	}
	return nil
}

func (c *Client) Users(ctx context.Context, tag string) ([]string, error) {
	resp, err := c.h.GetInboundUsers(ctx, &command.GetInboundUserRequest{Tag: tag})
	if err != nil {
		return nil, fmt.Errorf("list users of %s: %w", tag, err)
	}
	out := make([]string, 0, len(resp.Users))
	for _, u := range resp.Users {
		out = append(out, u.Email)
	}
	return out, nil
}
