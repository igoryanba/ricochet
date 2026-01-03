package bridge

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/igoryan-dao/ricochet/internal/bridge/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Client handles connection to Ricochet Cloud
type Client struct {
	cloudURL   string
	sessionID  string
	session    *yamux.Session
	grpcConn   *grpc.ClientConn
	stream     proto.BridgeService_ConnectClient
	incomingCh chan *proto.BridgeEvent
}

func NewClient(cloudURL, sessionID string) *Client {
	return &Client{
		cloudURL:   cloudURL,
		sessionID:  sessionID,
		incomingCh: make(chan *proto.BridgeEvent, 100),
	}
}

func (c *Client) Start(ctx context.Context) error {
	u, err := url.Parse(c.cloudURL)
	if err != nil {
		return err
	}

	log.Printf("Connecting to Ricochet Cloud at %s...", u.String())

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	rwc := NewWebSocketRWC(conn)

	// Start Yamux session
	session, err := yamux.Client(rwc, nil)
	if err != nil {
		return fmt.Errorf("yamux client: %w", err)
	}
	c.session = session

	// Setup gRPC over yamux stream
	c.grpcConn, err = grpc.DialContext(ctx, "",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return session.Open()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("grpc dial: %w", err)
	}

	client := proto.NewBridgeServiceClient(c.grpcConn)

	// Add auth secret to metadata
	secret := os.Getenv("RICOCHET_BRIDGE_SECRET")
	ctx = metadata.AppendToOutgoingContext(ctx, "x-bridge-secret", secret)

	c.stream, err = client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("bridge stream connect: %w", err)
	}

	// Initial heartbeat/auth
	err = c.stream.Send(&proto.BridgeEvent{
		SessionId: c.sessionID,
		Payload: &proto.BridgeEvent_Heartbeat{
			Heartbeat: &proto.Heartbeat{Timestamp: 0},
		},
	})
	if err != nil {
		return fmt.Errorf("initial handshake: %w", err)
	}

	// Start listening for incoming events
	go c.listenLoop()

	return nil
}

func (c *Client) listenLoop() {
	for {
		event, err := c.stream.Recv()
		if err != nil {
			log.Printf("Bridge stream closed: %v", err)
			return
		}
		c.incomingCh <- event
	}
}

func (c *Client) Send(event *proto.BridgeEvent) error {
	event.SessionId = c.sessionID
	return c.stream.Send(event)
}

func (c *Client) Incoming() <-chan *proto.BridgeEvent {
	return c.incomingCh
}

func (c *Client) Close() {
	if c.stream != nil {
		c.stream.CloseSend()
	}
	if c.grpcConn != nil {
		c.grpcConn.Close()
	}
	if c.session != nil {
		c.session.Close()
	}
}
