package noop

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNoop(t *testing.T) {
	want := errors.New("no backend specified, please specify a backend")
	_, _, got := Handler{}.Read(context.TODO(), nil)
	if diff := cmp.Diff(want.Error(), got.Error()); diff != "" {
		t.Fatal(diff)
	}
}
