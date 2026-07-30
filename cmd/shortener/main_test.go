package main

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestPrintBuildInfoUsesNAForEmptyValues(t *testing.T) {
	withBuildInfo("", "", "", func() {
		var buf bytes.Buffer

		printBuildInfo(&buf)

		want := "Build version: N/A\n" +
			"Build date: N/A\n" +
			"Build commit: N/A\n"
		if got := buf.String(); got != want {
			t.Fatalf("unexpected build info:\nwant:\n%s\ngot:\n%s", want, got)
		}
	})
}

func TestPrintBuildInfoUsesConfiguredValues(t *testing.T) {
	withBuildInfo("v1.2.3", "2026-07-02", "abc123", func() {
		var buf bytes.Buffer

		printBuildInfo(&buf)

		want := "Build version: v1.2.3\n" +
			"Build date: 2026-07-02\n" +
			"Build commit: abc123\n"
		if got := buf.String(); got != want {
			t.Fatalf("unexpected build info:\nwant:\n%s\ngot:\n%s", want, got)
		}
	})
}

func withBuildInfo(version, date, commit string, test func()) {
	oldVersion := buildVersion
	oldDate := buildDate
	oldCommit := buildCommit
	defer func() {
		buildVersion = oldVersion
		buildDate = oldDate
		buildCommit = oldCommit
	}()

	buildVersion = version
	buildDate = date
	buildCommit = commit
	test()
}

type fakeContextServer struct {
	run func(context.Context) error
}

func (s fakeContextServer) RunContext(ctx context.Context) error {
	return s.run(ctx)
}

func TestRunServersCancelsPeersWhenServerFails(t *testing.T) {
	wantErr := errors.New("listen failed")
	peerCanceled := make(chan struct{})

	err := runServers(
		context.Background(),
		fakeContextServer{
			run: func(context.Context) error {
				return wantErr
			},
		},
		fakeContextServer{
			run: func(ctx context.Context) error {
				<-ctx.Done()
				close(peerCanceled)
				return nil
			},
		},
	)

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected server error %v, got %v", wantErr, err)
	}
	select {
	case <-peerCanceled:
	case <-time.After(time.Second):
		t.Fatal("peer server was not canceled")
	}
}

func TestRunServersStopsCleanlyAfterContextCancellation(t *testing.T) {
	runContext, cancelRun := context.WithCancel(context.Background())
	serverStarted := make(chan struct{})
	runResult := make(chan error, 1)

	go func() {
		runResult <- runServers(
			runContext,
			fakeContextServer{
				run: func(ctx context.Context) error {
					close(serverStarted)
					<-ctx.Done()
					return nil
				},
			},
		)
	}()

	select {
	case <-serverStarted:
	case <-time.After(time.Second):
		t.Fatal("server did not start")
	}
	cancelRun()

	select {
	case err := <-runResult:
		if err != nil {
			t.Fatalf("unexpected shutdown error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("servers did not stop")
	}
}
