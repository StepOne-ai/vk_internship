package tests

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/StepOne-ai/vk_internship/internal/subpub"

	"github.com/stretchr/testify/require"
)

func TestSubscribePublish(t *testing.T) {
	sp := subpub.NewSubPub()
	defer sp.Close(context.Background())

	var received []string
	var mu sync.Mutex
	handler := func(msg any) {
		mu.Lock()
		received = append(received, msg.(string))
		mu.Unlock()
	}

	sub, err := sp.Subscribe("test", handler)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	err = sp.Publish("test", "msg1")
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond) // Даем время на обработку

	mu.Lock()
	require.Equal(t, []string{"msg1"}, received)
	mu.Unlock()
}

func TestUnsubscribe(t *testing.T) {
	sp := subpub.NewSubPub()
	defer sp.Close(context.Background())

	var received int
	handler := func(msg any) {
		received++
	}

	sub, _ := sp.Subscribe("test", handler)
	sub.Unsubscribe()

	sp.Publish("test", "msg")
	time.Sleep(100 * time.Millisecond)
	require.Zero(t, received)
}

func TestOrdering(t *testing.T) {
	sp := subpub.NewSubPub()
	defer sp.Close(context.Background())

	var msgs []string
	var wg sync.WaitGroup
	wg.Add(3)

	handler := func(msg any) {
		msgs = append(msgs, msg.(string))
		wg.Done()
	}

	sp.Subscribe("test", handler)

	sp.Publish("test", "first")
	sp.Publish("test", "second")
	sp.Publish("test", "third")

	wg.Wait()
	require.Equal(t, []string{"first", "second", "third"}, msgs)
}

func TestSlowSubscriber(t *testing.T) {
	sp := subpub.NewSubPub()
	defer sp.Close(context.Background())

	var fastReceived, slowReceived int
	var wg sync.WaitGroup
	wg.Add(2)

	fastHandler := func(msg any) {
		fastReceived++
		wg.Done()
	}

	slowHandler := func(msg any) {
		time.Sleep(200 * time.Millisecond)
		slowReceived++
		wg.Done()
	}

	sp.Subscribe("fast", fastHandler)
	sp.Subscribe("slow", slowHandler)

	start := time.Now()
	sp.Publish("fast", "msg1")
	sp.Publish("slow", "msg2")

	wg.Wait()
	duration := time.Since(start)

	require.Equal(t, 1, fastReceived)
	require.Equal(t, 1, slowReceived)
	require.Less(t, duration, 250*time.Millisecond, "Fast subscriber shouldn't wait for slow one")
}

func TestCloseWithTimeout(t *testing.T) {
	sp := subpub.NewSubPub()

	var wg sync.WaitGroup
	wg.Add(1)
	sp.Subscribe("test", func(msg any) {
		time.Sleep(500 * time.Millisecond)
		wg.Done()
	})

	sp.Publish("test", "msg")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := sp.Close(ctx)
	require.Error(t, err)
	require.Equal(t, context.DeadlineExceeded, err)

	wg.Wait() // Убедимся, что обработчик все же завершился
}

func TestPublishAfterClose(t *testing.T) {
	sp := subpub.NewSubPub()
	sp.Close(context.Background())

	err := sp.Publish("test", "msg")
	require.Error(t, err)
	require.Equal(t, subpub.ErrClosed, err)
}

func TestConcurrentOperations(t *testing.T) {
	sp := subpub.NewSubPub()
	defer sp.Close(context.Background())

	var received uint32
	var wg sync.WaitGroup
	wg.Add(100)

	handler := func(msg any) {
		atomic.AddUint32(&received, 1)
		wg.Done()
	}

	for i := 0; i < 10; i++ {
		go func() {
			sub, _ := sp.Subscribe("test", handler)
			defer sub.Unsubscribe()
		}()
	}

	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 10; i++ {
		go func() {
			sp.Publish("test", "msg")
		}()
	}

	wg.Wait()
	require.Equal(t, uint32(100), atomic.LoadUint32(&received))
}
