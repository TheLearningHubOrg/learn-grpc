package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	learningv1 "github.com/TheLearningHubOrg/learn-grpc/gen/learning/v1"
	"github.com/TheLearningHubOrg/learn-grpc/internal/roadmap"
)

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// dialBufconn starts the real gRPC server on an in-memory listener. Nothing
// binds a TCP port, so tests run in parallel without port clashes.
func dialBufconn(t *testing.T) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv, _ := NewGRPCServer(quietLogger())
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func testCtx(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestGetStage(t *testing.T) {
	t.Parallel()
	client := learningv1.NewLearningServiceClient(dialBufconn(t))

	stage, err := client.GetStage(testCtx(t), &learningv1.GetStageRequest{StageNumber: 3})
	if err != nil {
		t.Fatalf("GetStage: %v", err)
	}
	if stage.GetStageNumber() != 3 || stage.GetTitle() == "" || stage.GetTrack() != learningv1.Track_TRACK_FOUNDATIONS {
		t.Fatalf("unexpected stage: %v", stage)
	}
}

func TestGetStageErrors(t *testing.T) {
	t.Parallel()
	client := learningv1.NewLearningServiceClient(dialBufconn(t))

	for _, tc := range []struct {
		number int32
		want   codes.Code
	}{
		{0, codes.InvalidArgument},
		{-4, codes.InvalidArgument},
		{99, codes.NotFound},
	} {
		_, err := client.GetStage(testCtx(t), &learningv1.GetStageRequest{StageNumber: tc.number})
		if got := status.Code(err); got != tc.want {
			t.Errorf("GetStage(%d) code = %v, want %v (err %v)", tc.number, got, tc.want, err)
		}
	}
}

func collectRoadmap(t *testing.T, client learningv1.LearningServiceClient, track learningv1.Track) []*learningv1.Stage {
	t.Helper()
	stream, err := client.StreamRoadmap(testCtx(t), &learningv1.StreamRoadmapRequest{Track: track})
	if err != nil {
		t.Fatalf("StreamRoadmap: %v", err)
	}
	var out []*learningv1.Stage
	for {
		s, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatalf("Recv: %v", err)
		}
		out = append(out, s)
	}
}

func TestStreamRoadmap(t *testing.T) {
	t.Parallel()
	client := learningv1.NewLearningServiceClient(dialBufconn(t))

	all := collectRoadmap(t, client, learningv1.Track_TRACK_UNSPECIFIED)
	if len(all) != roadmap.Count() {
		t.Fatalf("got %d stages, want %d", len(all), roadmap.Count())
	}
	for i, s := range all {
		if s.GetStageNumber() != int32(i+1) {
			t.Fatalf("stage %d arrived at position %d: order must be preserved", s.GetStageNumber(), i)
		}
	}

	foundations := collectRoadmap(t, client, learningv1.Track_TRACK_FOUNDATIONS)
	if len(foundations) != 3 {
		t.Fatalf("got %d foundation stages, want 3", len(foundations))
	}
}

func TestStreamRoadmapCancelled(t *testing.T) {
	t.Parallel()
	client := learningv1.NewLearningServiceClient(dialBufconn(t))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before the call starts
	stream, err := client.StreamRoadmap(ctx, &learningv1.StreamRoadmapRequest{})
	if err == nil {
		_, err = stream.Recv()
	}
	if status.Code(err) != codes.Canceled {
		t.Fatalf("code = %v, want Canceled (err %v)", status.Code(err), err)
	}
}

func TestRecordMetrics(t *testing.T) {
	t.Parallel()
	client := learningv1.NewLearningServiceClient(dialBufconn(t))

	stream, err := client.RecordMetrics(testCtx(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []float64{4, 10, 1} {
		if err := stream.Send(&learningv1.MetricRecord{Name: "latency_ms", Value: v}); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}
	got, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv: %v", err)
	}
	if got.GetTotalRecords() != 3 || got.GetAverageValue() != 5 || got.GetMinValue() != 1 || got.GetMaxValue() != 10 {
		t.Fatalf("unexpected summary: %v", got)
	}
}

func TestRecordMetricsEmptyStream(t *testing.T) {
	t.Parallel()
	client := learningv1.NewLearningServiceClient(dialBufconn(t))

	stream, err := client.RecordMetrics(testCtx(t))
	if err != nil {
		t.Fatal(err)
	}
	got, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv: %v", err)
	}
	if got.GetTotalRecords() != 0 || got.GetMinValue() != 0 || got.GetMaxValue() != 0 {
		t.Fatalf("empty stream should give a zero summary, got %v", got)
	}
}

func TestRecordMetricsRejectsBadRecords(t *testing.T) {
	t.Parallel()
	client := learningv1.NewLearningServiceClient(dialBufconn(t))

	for _, rec := range []*learningv1.MetricRecord{
		{Name: "", Value: 1},
		{Name: "x", Value: math.NaN()},
		{Name: "x", Value: math.Inf(1)},
	} {
		stream, err := client.RecordMetrics(testCtx(t))
		if err != nil {
			t.Fatal(err)
		}
		// Send may return io.EOF if the server has already failed the RPC.
		// The status itself always comes from CloseAndRecv.
		if err := stream.Send(rec); err != nil && !errors.Is(err, io.EOF) {
			t.Fatalf("Send: %v", err)
		}
		_, err = stream.CloseAndRecv()
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("record %v: code = %v, want InvalidArgument", rec, status.Code(err))
		}
	}
}

func TestLiveExchange(t *testing.T) {
	t.Parallel()
	client := learningv1.NewLearningServiceClient(dialBufconn(t))

	stream, err := client.LiveExchange(testCtx(t))
	if err != nil {
		t.Fatal(err)
	}
	msgs := []string{"one", "two", "three"}
	for _, m := range msgs {
		if err := stream.Send(&learningv1.ChatMessage{Sender: "test", Message: m}); err != nil {
			t.Fatalf("Send: %v", err)
		}
		reply, err := stream.Recv()
		if err != nil {
			t.Fatalf("Recv: %v", err)
		}
		if want := "ack test: " + m; reply.GetMessage() != want || reply.GetSentAt() == nil {
			t.Fatalf("reply = %v, want message %q with a timestamp", reply, want)
		}
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); !errors.Is(err, io.EOF) {
		t.Fatalf("after CloseSend, Recv err = %v, want io.EOF", err)
	}
}

func TestHealthService(t *testing.T) {
	t.Parallel()
	health := healthpb.NewHealthClient(dialBufconn(t))

	for _, svc := range []string{"", "learning.v1.LearningService"} {
		resp, err := health.Check(testCtx(t), &healthpb.HealthCheckRequest{Service: svc})
		if err != nil {
			t.Fatalf("Check(%q): %v", svc, err)
		}
		if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
			t.Fatalf("Check(%q) = %v, want SERVING", svc, resp.GetStatus())
		}
	}
	_, err := health.Check(testCtx(t), &healthpb.HealthCheckRequest{Service: "no.such.Service"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("unknown service code = %v, want NotFound", status.Code(err))
	}
}

func TestUnaryRecovery(t *testing.T) {
	t.Parallel()
	interceptor := UnaryRecovery(quietLogger())
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test/Panic"},
		func(context.Context, any) (any, error) { panic("boom") })
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
}

func TestRoadmapReturnsCopies(t *testing.T) {
	t.Parallel()
	first := roadmap.Stages()
	first[0].Title = "mutated"
	first[0].KeyConcepts[0] = "mutated"
	again, _ := roadmap.Find(1)
	if again.GetTitle() == "mutated" || again.GetKeyConcepts()[0] == "mutated" {
		t.Fatal("roadmap.Stages leaked shared state")
	}
}
