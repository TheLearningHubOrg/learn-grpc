// Command client calls each of the four LearningService RPCs once.
//
//	go run ./cmd/server            # terminal 1
//	go run ./cmd/client -addr localhost:8080
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	learningv1 "github.com/TheLearningHubOrg/learn-grpc/gen/learning/v1"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "server address")
	flag.Parse()

	// NewClient does not connect yet. The first RPC triggers name
	// resolution and the HTTP/2 handshake.
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("create client: %v", err)
	}
	defer conn.Close()
	client := learningv1.NewLearningServiceClient(conn)

	// One deadline for the whole demo. Every RPC below inherits it.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := unary(ctx, client); err != nil {
		log.Fatal(err)
	}
	if err := serverStream(ctx, client); err != nil {
		log.Fatal(err)
	}
	if err := clientStream(ctx, client); err != nil {
		log.Fatal(err)
	}
	if err := bidi(ctx, client); err != nil {
		log.Fatal(err)
	}
}

func unary(ctx context.Context, c learningv1.LearningServiceClient) error {
	stage, err := c.GetStage(ctx, &learningv1.GetStageRequest{StageNumber: 1})
	if err != nil {
		return fmt.Errorf("GetStage: %w", err)
	}
	fmt.Printf("unary:   stage %d %q\n", stage.GetStageNumber(), stage.GetTitle())
	return nil
}

func serverStream(ctx context.Context, c learningv1.LearningServiceClient) error {
	stream, err := c.StreamRoadmap(ctx, &learningv1.StreamRoadmapRequest{Track: learningv1.Track_TRACK_FOUNDATIONS})
	if err != nil {
		return fmt.Errorf("StreamRoadmap: %w", err)
	}
	for {
		stage, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil // server returned OK
		}
		if err != nil {
			return fmt.Errorf("StreamRoadmap recv: %w", err)
		}
		fmt.Printf("stream:  stage %d %q\n", stage.GetStageNumber(), stage.GetTitle())
	}
}

func clientStream(ctx context.Context, c learningv1.LearningServiceClient) error {
	stream, err := c.RecordMetrics(ctx)
	if err != nil {
		return fmt.Errorf("RecordMetrics: %w", err)
	}
	for _, v := range []float64{12.5, 7.25, 30} {
		rec := &learningv1.MetricRecord{Name: "latency_ms", Value: v, RecordedAt: timestamppb.Now()}
		if err := stream.Send(rec); err != nil {
			// io.EOF here means the server already ended the RPC;
			// CloseAndRecv below returns the real status.
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
	}
	summary, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("RecordMetrics close: %w", err)
	}
	fmt.Printf("client:  %d records, avg %.2f, min %.2f, max %.2f\n",
		summary.GetTotalRecords(), summary.GetAverageValue(), summary.GetMinValue(), summary.GetMaxValue())
	return nil
}

func bidi(ctx context.Context, c learningv1.LearningServiceClient) error {
	stream, err := c.LiveExchange(ctx)
	if err != nil {
		return fmt.Errorf("LiveExchange: %w", err)
	}

	// Receive on its own goroutine: the directions are independent, and
	// a gRPC stream allows one concurrent sender and one concurrent receiver.
	done := make(chan error, 1)
	go func() {
		for {
			msg, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				done <- nil
				return
			}
			if err != nil {
				done <- err
				return
			}
			fmt.Printf("bidi:    %s\n", msg.GetMessage())
		}
	}()

	for _, text := range []string{"hello", "how are streams closed?"} {
		if err := stream.Send(&learningv1.ChatMessage{Sender: "client", Message: text, SentAt: timestamppb.Now()}); err != nil {
			return err
		}
	}
	if err := stream.CloseSend(); err != nil { // half-close: END_STREAM on our side
		return err
	}
	return <-done
}
