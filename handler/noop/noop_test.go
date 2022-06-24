package noop

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tonglil/buflogr"
)

func TestNoop_Handle(t *testing.T) {
	var buf bytes.Buffer
	n := &Handler{
		Log: buflogr.NewWithBuffer(&buf),
	}
	n.Handle(nil, nil, nil)
	want := "INFO no handler specified. please specify a handler\n"
	if diff := cmp.Diff(buf.String(), want); diff != "" {
		t.Fatalf(diff)
	}
}

func TestNoop_HandleSTDOUT(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	n := &Handler{}
	n.Handle(nil, nil, nil)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	want := `noop.go:22: "level"=0 "msg"="no handler specified. please specify a handler"` + "\n"
	if diff := cmp.Diff(buf.String(), want); diff != "" {
		t.Fatalf(diff)
	}
}
