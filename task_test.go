package alive_test

import (
	"os"
	"testing"

	"github.com/Drelf2018/alive"
)

func TestRunTask(t *testing.T) {
	err := alive.Task{Name: "cmd", Args: []string{"/C", "echo hello alive!"}}.Run(os.Stdout)
	if err != nil {
		t.Fatal(err)
	}
}
