package broker

import "testing"

func TestLastProgressSegment(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain line", "building '/nix/store/abc.drv'...", "building '/nix/store/abc.drv'..."},
		{"empty line", "", ""},
		{
			"git progress",
			"remote: Counting objects:   0% (1/1132)        \rremote: Counting objects:   1% (12/1132)        \rremote: Counting objects: 100% (1132/1132), done.",
			"remote: Counting objects: 100% (1132/1132), done.",
		},
		{"trailing carriage return", "a\rb\r", "b"},
		{"only carriage returns", "\r\r", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lastProgressSegment(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
