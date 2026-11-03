// Package roadmap holds the in-memory catalogue that LearningService serves.
package roadmap

import learningv1 "github.com/TheLearningHubOrg/learn-grpc/gen/learning/v1"

type stage struct {
	number   int32
	title    string
	focus    string
	concepts []string
	track    learningv1.Track
}

var stages = []stage{
	{1, "Foundations: the RPC Model, HTTP/2 & the Wire", "Channels, stubs, HTTP/2 streams, 5-byte framing",
		[]string{"grpc.NewClient", "HTTP/2 streams", "length-prefixed messages", "grpcurl"}, learningv1.Track_TRACK_FOUNDATIONS},
	{2, "Protocol Buffers: Schema Design & Code Generation", "proto3, editions, wire format, buf",
		[]string{"varint", "zigzag", "reserved", "buf breaking"}, learningv1.Track_TRACK_FOUNDATIONS},
	{3, "The Four RPC Patterns in Go", "Unary, server, client and bidirectional streaming",
		[]string{"SendAndClose", "CloseSend", "io.EOF", "bufconn"}, learningv1.Track_TRACK_FOUNDATIONS},
	{4, "Interceptors, Metadata, Deadlines & Errors", "Middleware chains, context propagation, status codes",
		[]string{"ChainUnaryInterceptor", "metadata", "grpc-timeout", "errdetails"}, learningv1.Track_TRACK_PRODUCTION},
	{5, "Resilience, Name Resolution & Load Balancing", "Service config, retries, round_robin, proxies",
		[]string{"retryPolicy", "dns:///", "headless Service", "Envoy"}, learningv1.Track_TRACK_PRODUCTION},
	{6, "Security, Health, Observability & Deployment", "mTLS, health checking, reflection, OpenTelemetry",
		[]string{"credentials.NewTLS", "grpc.health.v1", "reflection", "otelgrpc"}, learningv1.Track_TRACK_PRODUCTION},
	{7, "Staff Interview Handbook", "Scenario questions and a one-page cheatsheet",
		[]string{"failure modes", "trade-offs", "migration strategy"}, learningv1.Track_TRACK_CAREER},
}

// Count is the number of stages in the catalogue.
func Count() int { return len(stages) }

// Stages returns fresh messages on every call, so a caller that mutates a
// result cannot corrupt the shared catalogue.
func Stages() []*learningv1.Stage {
	out := make([]*learningv1.Stage, 0, len(stages))
	for _, s := range stages {
		out = append(out, s.toProto())
	}
	return out
}

// Find returns the stage with the given number.
func Find(number int32) (*learningv1.Stage, bool) {
	for _, s := range stages {
		if s.number == number {
			return s.toProto(), true
		}
	}
	return nil, false
}

func (s stage) toProto() *learningv1.Stage {
	return &learningv1.Stage{
		StageNumber: s.number,
		Title:       s.title,
		DomainFocus: s.focus,
		KeyConcepts: append([]string(nil), s.concepts...),
		Track:       s.track,
	}
}
