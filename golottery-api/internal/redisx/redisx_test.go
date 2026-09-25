package redisx

import (
	"context"
	"testing"
)

func TestOpenAndPing(t *testing.T) {
	client := OpenTest(t)
	if err := Ping(context.Background(), client); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if err := Ping(context.Background(), Unreachable()); err == nil {
		t.Fatal("Ping() on a closed port succeeded")
	}
	if _, err := Open(context.Background(), ""); err == nil {
		t.Fatal("Open() accepted an empty URL")
	}
	if _, err := Open(context.Background(), "not a url"); err == nil {
		t.Fatal("Open() accepted a malformed URL")
	}
}
