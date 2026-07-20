package httpserver_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/config"
	httpserver "github.com/timaogurtzova/shortener/internal/http"
)

func TestRunContextWaitsForActiveRequest(t *testing.T) {
	address := reserveTCPAddress(t)
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-releaseRequest:
		default:
			close(releaseRequest)
		}
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		<-releaseRequest
		_, _ = w.Write([]byte("completed"))
	})
	cfg := &config.Configuration{
		Server: config.ServerConfiguration{
			Address:      address,
			IdleTimeout:  time.Second,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		},
	}
	server := httpserver.NewServer(cfg, handler)

	runCtx, cancelRun := context.WithCancel(context.Background())
	t.Cleanup(cancelRun)
	runResult := make(chan error, 1)
	go func() {
		runResult <- server.RunContext(runCtx)
	}()

	waitForTCPServer(t, address)
	responseResult := make(chan string, 1)
	responseError := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + address)
		if err != nil {
			responseError <- err
			return
		}
		defer response.Body.Close()

		body, err := io.ReadAll(response.Body)
		if err != nil {
			responseError <- err
			return
		}
		responseResult <- string(body)
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("request did not reach the handler")
	}
	cancelRun()

	select {
	case err := <-runResult:
		t.Fatalf("server returned before active request completed: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseRequest)
	select {
	case err := <-responseError:
		t.Fatalf("request failed during graceful shutdown: %v", err)
	case body := <-responseResult:
		assert.Equal(t, "completed", body)
	case <-time.After(time.Second):
		t.Fatal("active request was not completed")
	}

	select {
	case err := <-runResult:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("server did not stop after active request completed")
	}
}

func reserveTCPAddress(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}

func waitForTCPServer(t *testing.T, address string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", address, 20*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server %s did not start", address)
}
