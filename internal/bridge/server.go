package bridge

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/igoryan-dao/ricochet/internal/bridge/proto"
	"google.golang.org/grpc"
)

// Server is a mockup of the Ricochet Cloud part for testing
type Server struct {
	proto.UnimplementedBridgeServiceServer
	upgrader websocket.Upgrader
	port     int
}

func NewServer(port int) *Server {
	return &Server{
		port: port,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *Server) Start(ctx context.Context) error {
	http.HandleFunc("/ws", s.handleWebSocket)

	server := &http.Server{Addr: fmt.Sprintf(":%d", s.port)}

	log.Printf("Bridge Test Server starting on :%d...", s.port)

	go func() {
		<-ctx.Done()
		server.Close()
	}()

	return server.ListenAndServe()
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v", err)
		return
	}

	rwc := NewWebSocketRWC(conn)

	// Start Yamux session
	session, err := yamux.Server(rwc, nil)
	if err != nil {
		log.Printf("Yamux server error: %v", err)
		return
	}

	// Start gRPC server over yamux session
	grpcServer := grpc.NewServer()
	proto.RegisterBridgeServiceServer(grpcServer, s)

	if err := grpcServer.Serve(session); err != nil {
		log.Printf("gRPC server error: %v", err)
	}
}

func (s *Server) Connect(stream proto.BridgeService_ConnectServer) error {
	log.Println("New bridge client connected via gRPC stream")
	for {
		event, err := stream.Recv()
		if err != nil {
			log.Printf("Stream closed: %v", err)
			return err
		}

		log.Printf("Server received event from session %s: %v", event.SessionId, event.Payload)

		// Echo for test
		if err := stream.Send(event); err != nil {
			return err
		}
	}
}
