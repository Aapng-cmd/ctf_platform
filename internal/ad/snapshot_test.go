package ad

import (
	"bytes"
	"testing"
)

func TestScrubPublicBoard(t *testing.T) {
	raw := []byte(`{"teams":[{"name":"</script>","host":"10.0.0.1","services":[{"message":"FLAG{x}","status":"DOWN"}]}]}`)
	out := ScrubPublicBoard(raw)
	if bytes.Contains(out, []byte("10.0.0.1")) || bytes.Contains(out, []byte("FLAG{x}")) || bytes.Contains(out, []byte("</script>")) {
		t.Fatalf("public board still contains private data: %s", out)
	}
	if !bytes.Contains(out, []byte(`\u003c`)) || !bytes.Contains(out, []byte(`"status":"DOWN"`)) {
		t.Fatalf("board lost the public status: %s", out)
	}
}
