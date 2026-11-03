package server

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	learningv1 "github.com/TheLearningHubOrg/learn-grpc/gen/learning/v1"
)

// startHTTP runs the production handler on a real loopback port, so these
// tests cover the h2c path that the Docker image uses.
func startHTTP(t *testing.T) (addr string, setStatus func(healthpb.HealthCheckResponse_ServingStatus)) {
	t.Helper()
	grpcServer, hs := NewGRPCServer(quietLogger())
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := NewHTTPServer(lis.Addr().String(), NewHandler(grpcServer, hs))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { _ = srv.Close(); grpcServer.Stop() })
	return lis.Addr().String(), func(s healthpb.HealthCheckResponse_ServingStatus) { hs.SetServingStatus("", s) }
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, strings.TrimSpace(string(body))
}

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()
	addr, setStatus := startHTTP(t)

	if code, body := get(t, "http://"+addr+"/health"); code != http.StatusOK || body != `{"status":"SERVING"}` {
		t.Fatalf("GET /health = %d %s", code, body)
	}

	setStatus(healthpb.HealthCheckResponse_NOT_SERVING)
	if code, _ := get(t, "http://"+addr+"/health"); code != http.StatusServiceUnavailable {
		t.Fatalf("GET /health while draining = %d, want 503", code)
	}
}

func TestUnknownHTTPPath(t *testing.T) {
	t.Parallel()
	addr, _ := startHTTP(t)
	if code, _ := get(t, "http://"+addr+"/nope"); code != http.StatusNotFound {
		t.Fatalf("GET /nope = %d, want 404", code)
	}
}

func TestGRPCOverSharedPort(t *testing.T) {
	t.Parallel()
	addr, _ := startHTTP(t)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	stage, err := learningv1.NewLearningServiceClient(conn).GetStage(testCtx(t), &learningv1.GetStageRequest{StageNumber: 1})
	if err != nil {
		t.Fatalf("GetStage over h2c: %v", err)
	}
	if stage.GetStageNumber() != 1 {
		t.Fatalf("unexpected stage %v", stage)
	}
}
