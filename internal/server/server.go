package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	learningv1 "github.com/TheLearningHubOrg/learn-grpc/gen/learning/v1"
)

// NewGRPCServer builds the gRPC server with interceptors, the learning
// service, the standard health service and server reflection.
func NewGRPCServer(logger *slog.Logger) (*grpc.Server, *health.Server) {
	srv := grpc.NewServer(
		// The first interceptor listed is the outermost: recovery wraps
		// logging, so a panic is still logged with its status code.
		grpc.ChainUnaryInterceptor(UnaryRecovery(logger), UnaryLogger(logger)),
		grpc.ChainStreamInterceptor(StreamRecovery(logger), StreamLogger(logger)),
	)
	learningv1.RegisterLearningServiceServer(srv, NewLearningService())

	hs := health.NewServer() // reports SERVING for "" (the whole server) by default
	hs.SetServingStatus(learningv1.LearningService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, hs)

	reflection.Register(srv) // lets grpcurl list and describe services
	return srv, hs
}

// NewHandler serves gRPC and plain HTTP on one port. A request is gRPC when
// it arrives over HTTP/2 with a content-type starting with application/grpc;
// everything else (curl, load balancer probes) goes to the HTTP mux.
func NewHandler(grpcServer *grpc.Server, hs *health.Server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp, err := hs.Check(r.Context(), &healthpb.HealthCheckRequest{})
		if err != nil || resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"status":"NOT_SERVING"}`)
			return
		}
		fmt.Fprint(w, `{"status":"SERVING"}`)
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// NewHTTPServer accepts HTTP/1.1 and cleartext HTTP/2 with prior knowledge
// (h2c), which is what a gRPC client sends when it uses insecure credentials.
// Put TLS in front (a mesh sidecar or ingress) or switch to ListenAndServeTLS
// before exposing this beyond a private network.
func NewHTTPServer(addr string, h http.Handler) *http.Server {
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		Protocols:         &protocols,
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: it would cut long-lived streams. Deadlines are
		// set per RPC by clients instead.
	}
}
