package tests

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/StepOne-ai/vk_internship/cmd"
	"github.com/StepOne-ai/vk_internship/internal/logger"
	"github.com/StepOne-ai/vk_internship/internal/subpub"
	pb "github.com/StepOne-ai/vk_internship/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

var (
	lis  *bufconn.Listener
	sp   subpub.SubPub
	logg logger.Logger
	once sync.Once
)

func setupTest() {
	once.Do(func() {
		lis = bufconn.Listen(bufSize)
		sp = subpub.NewSubPub()
		logger.SetupLogger(logger.DebugLevel)
		logg = logger.With(logger.Field{Key: "component", Value: "grpc-test"})

		server := grpc.NewServer()
		pb.RegisterPubSubServer(server, cmd.NewPubSubServer(sp, logg))

		go func() {
			if err := server.Serve(lis); err != nil {
				panic(err)
			}
		}()
	})
}

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestGRPC(t *testing.T) {
	setupTest()

	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := pb.NewPubSubClient(conn)

	t.Run("Publish and Subscribe", func(t *testing.T) {
		key := "test_key"
		data := "test_data"

		stream, err := client.Subscribe(ctx, &pb.SubscribeRequest{Key: key})
		require.NoError(t, err)

		_, err = client.Publish(ctx, &pb.PublishRequest{
			Key:  key,
			Data: data,
		})
		require.NoError(t, err)

		event, err := stream.Recv()
		require.NoError(t, err)
		assert.Equal(t, data, event.Data)
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stream, err := client.Subscribe(ctx, &pb.SubscribeRequest{Key: "cancel_test"})
		require.NoError(t, err)

		cancel()

		_, err = stream.Recv()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("Server Shutdown", func(t *testing.T) {
		go func() {
			time.Sleep(100 * time.Millisecond)
			lis.Close()
		}()

		_, err := client.Publish(ctx, &pb.PublishRequest{
			Key:  "shutdown_test",
			Data: "data",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection error")
	})
}
