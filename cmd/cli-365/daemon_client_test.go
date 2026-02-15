package main

import (
	"reflect"
	"testing"
)

func TestStripDaemonFlag(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "standalone flag",
			in:   []string{"--daemon", "auth", "status"},
			want: []string{"auth", "status"},
		},
		{
			name: "equals true",
			in:   []string{"--daemon=true", "mail", "search", "x"},
			want: []string{"mail", "search", "x"},
		},
		{
			name: "value token",
			in:   []string{"--daemon", "true", "calendar", "list"},
			want: []string{"calendar", "list"},
		},
		{
			name: "mixed args",
			in:   []string{"--config", "x.yaml", "--daemon", "auth", "status"},
			want: []string{"--config", "x.yaml", "auth", "status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripDaemonFlag(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("stripDaemonFlag(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
