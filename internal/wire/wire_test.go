package wire

import (
	"bytes"
	"errors"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	learningv1 "github.com/TheLearningHubOrg/learn-grpc/gen/learning/v1"
)

func TestAppendVarint(t *testing.T) {
	cases := []struct {
		in   uint64
		want []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{127, []byte{0x7f}},
		{128, []byte{0x80, 0x01}},
		{150, []byte{0x96, 0x01}},
		{16384, []byte{0x80, 0x80, 0x01}},
	}
	for _, c := range cases {
		got := AppendVarint(nil, c.in)
		if !bytes.Equal(got, c.want) {
			t.Errorf("AppendVarint(%d) = % x, want % x", c.in, got, c.want)
		}
		if lib := protowire.AppendVarint(nil, c.in); !bytes.Equal(got, lib) {
			t.Errorf("AppendVarint(%d) = % x, protowire gives % x", c.in, got, lib)
		}
	}
}

func TestNegativeInt32TakesTenBytes(t *testing.T) {
	// int32 -1 is sign-extended to 64 bits before varint encoding.
	var minusOne int32 = -1
	if n := len(AppendVarint(nil, uint64(int64(minusOne)))); n != 10 {
		t.Fatalf("int32(-1) encoded in %d bytes, want 10", n)
	}
	if n := len(AppendVarint(nil, uint64(ZigZag32(minusOne)))); n != 1 {
		t.Fatalf("sint32(-1) encoded in %d bytes, want 1", n)
	}
}

func TestZigZag(t *testing.T) {
	for in, want := range map[int32]uint32{0: 0, -1: 1, 1: 2, -2: 3, 2: 4, 2147483647: 4294967294, -2147483648: 4294967295} {
		if got := ZigZag32(in); got != want {
			t.Errorf("ZigZag32(%d) = %d, want %d", in, got, want)
		}
		if got := protowire.EncodeZigZag(int64(in)); got != uint64(want) {
			t.Errorf("protowire.EncodeZigZag(%d) = %d, want %d", in, got, want)
		}
	}
	if got := ZigZag64(-3); got != 5 {
		t.Errorf("ZigZag64(-3) = %d, want 5", got)
	}
}

func TestHandEncodingMatchesProtoMarshal(t *testing.T) {
	// GetStageRequest{stage_number: 150}: field 1, wire type 0, value 150.
	want, err := proto.Marshal(&learningv1.GetStageRequest{StageNumber: 150})
	if err != nil {
		t.Fatal(err)
	}
	got := AppendVarint(AppendVarint(nil, Tag(1, TypeVarint)), 150)
	if !bytes.Equal(got, want) {
		t.Fatalf("hand encoding % x, proto.Marshal % x", got, want)
	}
	if !bytes.Equal(got, []byte{0x08, 0x96, 0x01}) {
		t.Fatalf("unexpected bytes % x", got)
	}
}

func TestDefaultValuesAreNotEncoded(t *testing.T) {
	b, err := proto.Marshal(&learningv1.GetStageRequest{StageNumber: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 0 {
		t.Fatalf("zero-valued message encoded to % x, want no bytes", b)
	}
}

func TestStringFieldIsLengthDelimited(t *testing.T) {
	// ChatMessage{sender: "hi"}: tag (1<<3|2)=0x0a, length 2, then the bytes.
	b, err := proto.Marshal(&learningv1.ChatMessage{Sender: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0x0a, 0x02, 'h', 'i'}; !bytes.Equal(b, want) {
		t.Fatalf("got % x, want % x", b, want)
	}
}

func TestFrameRoundTrip(t *testing.T) {
	a, b := []byte{0x08, 0x01}, []byte{0x08, 0x02}
	stream := append(Frame(a, false), Frame(b, true)...)
	if !bytes.Equal(stream[:5], []byte{0, 0, 0, 0, 2}) {
		t.Fatalf("prefix % x, want 00 00 00 00 02", stream[:5])
	}

	p1, c1, rest, err := Unframe(stream)
	if err != nil || c1 || !bytes.Equal(p1, a) {
		t.Fatalf("first message: % x compressed=%v err=%v", p1, c1, err)
	}
	p2, c2, rest, err := Unframe(rest)
	if err != nil || !c2 || !bytes.Equal(p2, b) || len(rest) != 0 {
		t.Fatalf("second message: % x compressed=%v rest=%d err=%v", p2, c2, len(rest), err)
	}
}

func TestUnframeShort(t *testing.T) {
	for _, b := range [][]byte{nil, {0, 0, 0}, {0, 0, 0, 0, 9, 1, 2}} {
		if _, _, _, err := Unframe(b); !errors.Is(err, ErrShortFrame) {
			t.Errorf("Unframe(% x) err = %v, want ErrShortFrame", b, err)
		}
	}
}
