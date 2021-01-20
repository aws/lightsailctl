package cs

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/jsonmessage"
)

func TestExtractDigest(t *testing.T) {
	got := ""
	badAux := json.RawMessage("42")
	extractDigest(&got)(jsonmessage.JSONMessage{Aux: &badAux})
	if got != "" {
		t.Errorf("unexpected got: %q", got)
	}
	wantDigest := "sha256:b95cf9b496720e43b12ce435775d5e337a6648147825c0fc8fc0ff93616c69a0"
	goodAux := json.RawMessage(`{"digest": "` + wantDigest + `"}`)
	extractDigest(&got)(jsonmessage.JSONMessage{Aux: &goodAux})
	if got != wantDigest {
		t.Errorf("got: %q", got)
		t.Logf("want: %q", wantDigest)
	}
}

func Example_skipStatuses() {
	r := skipStatuses(
		strings.NewReader(`
		{"status": "keep me"}
		{"status": "xyz skip1 abc"}
		{"status": "also keep me!"}
		{"status": "\tskip2"}`),
		"skip2", "skip1",
	)
	if _, err := io.Copy(os.Stdout, r); err != nil {
		fmt.Println(err)
		return
	}
	// Output:
	// {"status":"keep me"}
	// {"status":"also keep me!"}
}
