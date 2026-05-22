package client

import (
	"reflect"
	"testing"
)

func TestMapToDotenv_SortedAndStable(t *testing.T) {
	got := MapToDotenv(map[string]string{"B": "2", "A": "1"})
	if got != "A=1\nB=2" {
		t.Errorf("MapToDotenv() = %q, want %q", got, "A=1\nB=2")
	}
}

func TestDotenvToMap(t *testing.T) {
	got := DotenvToMap("A=1\nB=hello world\n\n# comment\nC=")
	want := map[string]string{"A": "1", "B": "hello world", "C": ""}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DotenvToMap() = %#v, want %#v", got, want)
	}
}

func TestMapToDotenv_Empty(t *testing.T) {
	if got := MapToDotenv(nil); got != "" {
		t.Errorf("MapToDotenv(nil) = %q, want empty", got)
	}
}
