package bot_test

import (
	"testing"

	"feedwatch/internal/bot"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		input string
		name  string
		args  []string
		ok    bool
	}{
		{"/new_pipeline my-feed", "new_pipeline", []string{"my-feed"}, true},
		{"/add_source my-feed @golang_jobs", "add_source", []string{"my-feed", "@golang_jobs"}, true},
		{"/set_output my-feed @my_channel", "set_output", []string{"my-feed", "@my_channel"}, true},
		{"/add_run my-feed golang", "add_run", []string{"my-feed", "golang"}, true},
		{"/add_stop my-feed junior", "add_stop", []string{"my-feed", "junior"}, true},
		{"/del_pipeline my-feed", "del_pipeline", []string{"my-feed"}, true},
		{"/list", "list", []string{}, true},
		{"hello world", "", nil, false},
		{"", "", nil, false},
		{"/", "", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, args, ok := bot.ParseCommand(tt.input)
			if ok != tt.ok {
				t.Fatalf("ParseCommand(%q): ok=%v, want %v", tt.input, ok, tt.ok)
			}
			if !ok {
				return
			}
			if name != tt.name {
				t.Errorf("name=%q, want %q", name, tt.name)
			}
			if len(args) != len(tt.args) {
				t.Errorf("args=%v, want %v", args, tt.args)
				return
			}
			for i := range args {
				if args[i] != tt.args[i] {
					t.Errorf("args[%d]=%q, want %q", i, args[i], tt.args[i])
				}
			}
		})
	}
}
