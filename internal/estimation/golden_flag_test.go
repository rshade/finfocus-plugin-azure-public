package estimation

import (
	"flag"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if flag.Lookup("update-golden") == nil {
		flag.Bool("update-golden", false, "accepted so go test ./... can pass -update-golden")
	}
	os.Exit(m.Run())
}
