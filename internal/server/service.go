// Package server implements learning.v1.LearningService and wires it into a
// gRPC server and an HTTP handler that share one port.
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	learningv1 "github.com/TheLearningHubOrg/learn-grpc/gen/learning/v1"
	"github.com/TheLearningHubOrg/learn-grpc/internal/roadmap"
)

// LearningService implements the four RPCs declared in learning.proto.
type LearningService struct {
	// Embedding the Unimplemented struct by value keeps the type compiling
	// when new RPCs are added to the proto: they return codes.Unimplemented
	// until a real method is written.
	learningv1.UnimplementedLearningServiceServer

	now func() time.Time
}

// NewLearningService returns a service that stamps replies with time.Now.
func NewLearningService() *LearningService {
	return &LearningService{now: time.Now}
}

// GetStage is the unary RPC: validate, look up, return.
func (s *LearningService) GetStage(_ context.Context, req *learningv1.GetStageRequest) (*learningv1.Stage, error) {
	n := req.GetStageNumber()
	if n < 1 {
		return nil, status.Errorf(codes.InvalidArgument, "stage_number must be >= 1, got %d", n)
	}
	stage, ok := roadmap.Find(n)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "stage %d does not exist", n)
	}
	return stage, nil
}

// StreamRoadmap is the server-streaming RPC. Returning nil ends the stream
// with status OK; the client sees io.EOF after the last message.
func (s *LearningService) StreamRoadmap(req *learningv1.StreamRoadmapRequest, stream grpc.ServerStreamingServer[learningv1.Stage]) error {
	for _, stage := range roadmap.Stages() {
		if req.GetTrack() != learningv1.Track_TRACK_UNSPECIFIED && stage.GetTrack() != req.GetTrack() {
			continue
		}
		// Stop work as soon as the caller cancels or its deadline passes.
		if err := stream.Context().Err(); err != nil {
			return status.FromContextError(err).Err()
		}
		if err := stream.Send(stage); err != nil {
			return err
		}
	}
	return nil
}

// RecordMetrics is the client-streaming RPC. It reads until io.EOF, which
// means the client called CloseSend, then replies once with SendAndClose.
func (s *LearningService) RecordMetrics(stream grpc.ClientStreamingServer[learningv1.MetricRecord, learningv1.MetricSummary]) error {
	var (
		count    int32
		sum      float64
		min, max = math.Inf(1), math.Inf(-1)
	)
	for {
		rec, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			summary := &learningv1.MetricSummary{TotalRecords: count}
			if count > 0 {
				summary.AverageValue = sum / float64(count)
				summary.MinValue = min
				summary.MaxValue = max
			}
			return stream.SendAndClose(summary)
		}
		if err != nil {
			return err
		}
		if rec.GetName() == "" {
			return status.Error(codes.InvalidArgument, "metric name is required")
		}
		v := rec.GetValue()
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return status.Errorf(codes.InvalidArgument, "metric %q has a non-finite value", rec.GetName())
		}
		count++
		sum += v
		min = math.Min(min, v)
		max = math.Max(max, v)
	}
}

// LiveExchange is the bidirectional RPC. This handler answers each message as
// it arrives; the two directions are independent, so a server could also
// send without waiting for input.
func (s *LearningService) LiveExchange(stream grpc.BidiStreamingServer[learningv1.ChatMessage, learningv1.ChatMessage]) error {
	for {
		in, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		reply := &learningv1.ChatMessage{
			Sender:  "learn-grpc",
			Message: fmt.Sprintf("ack %s: %s", in.GetSender(), in.GetMessage()),
			SentAt:  timestamppb.New(s.now()),
		}
		if err := stream.Send(reply); err != nil {
			return err
		}
	}
}
