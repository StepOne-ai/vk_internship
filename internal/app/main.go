package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/StepOne-ai/vk_internship/internal/logger"
	"github.com/StepOne-ai/vk_internship/internal/subpub"
	pb "github.com/StepOne-ai/vk_internship/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Config struct {
	Port int `mapstructure:"PORT"`
}

type PubSubServer struct {
	pb.UnimplementedPubSubServer // Исправлено имя
	sp                           subpub.SubPub
	logger                       logger.Logger
}

func NewPubSubServer(sp subpub.SubPub, logger logger.Logger) *PubSubServer {
	return &PubSubServer{
		sp:     sp,
		logger: logger,
	}
}

func (s *PubSubServer) Subscribe(req *pb.SubscribeRequest, stream pb.PubSub_SubscribeServer) error {
	ch := make(chan any, 10)
	sub, err := s.sp.Subscribe(req.Key, func(msg any) {
		ch <- msg
	})
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	defer sub.Unsubscribe()

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	go func() {
		<-ctx.Done()
		close(ch)
	}()

	for msg := range ch {
		event := &pb.Event{
			Data: msg.(string),
		}
		if err := stream.Send(event); err != nil {
			s.logger.Error(ctx, "Failed to send event", err, logger.Field{Key: "msg", Value: msg})
			return status.Error(codes.Internal, "stream send error")
		}
	}

	return nil
}

func (s *PubSubServer) Publish(ctx context.Context, req *pb.PublishRequest) (*emptypb.Empty, error) {
	if err := s.sp.Publish(req.Key, req.Data); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &emptypb.Empty{}, nil
}

func main() {
	cfg := Config{Port: 8080}
	logger.SetupLogger(logger.InfoLevel)
	log := logger.With(logger.Field{Key: "component", Value: "grpc-server"})

	sp := subpub.NewSubPub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		sp.Close(ctx)
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		log.Error(context.Background(), "Failed to listen", err, logger.Field{Key: "port", Value: cfg.Port})
	}

	server := grpc.NewServer()
	pb.RegisterPubSubServer(server, NewPubSubServer(sp, log))

	go func() {
		log.Info(context.Background(), "Starting server", logger.Field{Key: "port", Value: cfg.Port})
		if err := server.Serve(lis); err != nil {
			log.Error(context.Background(), "Server error", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	server.GracefulStop()
}
