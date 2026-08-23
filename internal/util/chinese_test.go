package util

import "testing"

func TestTraditionalizeChineseName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simplified", input: "无码女优", want: "無碼女優"},
		{name: "traditional", input: "無碼女優", want: "無碼女優"},
		{name: "trim whitespace", input: "  无码  ", want: "無碼"},
		{name: "empty", input: "  ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TraditionalizeChineseName(tt.input)
			if err != nil {
				t.Fatalf("TraditionalizeChineseName(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("TraditionalizeChineseName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
