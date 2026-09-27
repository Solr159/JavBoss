package util_test

import (
	"slices"
	"strings"
	"testing"

	"javboss/internal/util"
)

func TestExtractCodeFromName(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "1024核工厂-ABP-356.HD.mp4",
			expected: []string{"ABP-356"},
		},
		{
			input:    "ABP-178.avi",
			expected: []string{"ABP-178"},
		},
		{
			input:    "ABP-888C.mp4",
			expected: []string{"ABP-888", "ABP-888C"},
		},
		{
			input:    "abp-782_1.mp4",
			expected: []string{"ABP-782"},
		},
		{
			input:    "AGEMIX-080.avi",
			expected: []string{"AGEMIX-080"},
		},
		{
			input:    "AMBI027a.avi",
			expected: []string{"AMBI-027", "AMBI-027A"},
		},
		{
			input:    "[44x.me]AOZ-132-C.mp4",
			expected: []string{"AOZ-132"},
		},
		{
			input:    "IBW-938z.mp4",
			expected: []string{"IBW-938", "IBW-938Z"},
		},
		{
			input:    "BANK-002 S .mp4",
			expected: []string{"BANK-002"},
		},
		{
			input:    "dv-1530_0.avi",
			expected: []string{"DV-1530"},
		},
		{
			input:    "dv-1448.mp4",
			expected: []string{"DV-1448"},
		},
		{
			input:    "SDNM256.mp4",
			expected: []string{"SDNM-256", "SDNM256"},
		},
		{
			input:    "PKPD066C.mp4",
			expected: []string{"PKPD-066", "PKPD-066C"},
		},
		{
			input:    "miaa00068.mp4",
			expected: []string{"MIAA-068", "MIAA00068", "MIAA-00068"},
		},
		{
			input:    "ovg00129.mp4",
			expected: []string{"OVG-129", "OVG00129", "OVG-00129"},
		},
		{
			input:    "[XJ0610.com]IBW-497z-720p.mp4",
			expected: []string{"XJ-610", "IBW-497", "IBW-497Z", "XJ0610", "XJ-0610"},
		},
		{
			input:    "IBW572Z.mp4",
			expected: []string{"IBW-572", "IBW-572Z"},
		},
		{
			input:    "Heyzo-0945-HD.mp4",
			expected: []string{"HEYZO-945", "HEYZO-0945"},
		},
		{
			input:    "Heyzo - 0945 - Reiko Kobayakawa (小早川怜子).mp4",
			expected: []string{"HEYZO-0945"},
		},
		{
			input:    "HEYZO 0945 美痴女～爆乳弁護士に責められる～ - 小早川怜子 [UNCENSORED].mp4",
			expected: []string{"HEYZO-945", "HEYZO0945", "HEYZO-0945"},
		},
		{
			input:    "Heyzo-0945-HD.mp4",
			expected: []string{"HEYZO-945", "HEYZO-0945"},
		},
		{
			input:    "Tokyo Hot n0646.avi",
			expected: []string{"N0646"},
		},
		{
			input:    "051626-001-CARIB.mp4",
			expected: []string{"051626-001"},
		},
		{
			input:    "031419_001-1pon-1080p.mp4",
			expected: []string{"PON-1080", "PON-1080P", "031419_001"},
		},
		{
			input:    "heydouga-4030-2296.mp4",
			expected: []string{"YDOUGA-4030", "HEYDOUGA-4030", "4030-2296"},
		},
		{
			input:    "PT-82.mp4",
			expected: []string{"PT-082", "PT-82"},
		},
		{
			input:    "LLDV-44.mp4",
			expected: []string{"LLDV-044", "LLDV-44"},
		},
		{
			input:    "t28-502.mp4",
			expected: []string{"T28-502", "T28"},
		},
		{
			input:    "javcn_MCB3DBD-42-H265.mp4",
			expected: []string{"DBD-042", "MCB3DBD-42", "H265"},
		},
	}

	for _, tt := range tests {
		got := util.ExtractCodeFromName(tt.input)
		if !slices.Equal(got, tt.expected) {
			t.Fatalf("ExtractCodeFromName(%q) = %#v, want %#v", tt.input, got, tt.expected)
		}
	}
}

func TestExtractFC2PPVCodesFromName(t *testing.T) {
	for _, tc := range []struct {
		filename string
		want     []string
	}{
		{"102090.Xyz@FC2-PPV-4808856.mp4", []string{"FC2-PPV-4808856"}},
		{"FC2-PPV-4983452.mp4", []string{"FC2-PPV-4983452"}},
		{"FC2PPV-4810487.mp4", []string{"FC2-PPV-4810487"}},
		{"hhd800.com@FC2-PPV-2608885.mp4", []string{"FC2-PPV-2608885"}},
		{"【标题】fc2ppv3101826 18岁 标题.mp4", []string{"FC2-PPV-3101826"}},
		{"/videos/FC2_PPV_1234567_1.mp4", []string{"FC2-PPV-1234567"}},
		{"FC2 PPV 123456789-C.mp4", []string{"FC2-PPV-123456789"}},
		{"FC2-PPV-0000123.mp4", []string{"FC2-PPV-0000123"}},
		{"FC2-PPV-1.mp4", []string{"FC2-PPV-1"}},
		{"FC2-PPV-1234567 fc2ppv1234567 FC2PPV7654321.mp4", []string{"FC2-PPV-1234567", "FC2-PPV-7654321"}},
	} {
		t.Run(tc.filename, func(t *testing.T) {
			if got := util.ExtractCodeFromName(tc.filename); !slices.Equal(got, tc.want) {
				t.Fatalf("codes = %v, want %v", got, tc.want)
			}
			if got := util.ExtractUncensoredCodesFromName(tc.filename); !slices.Equal(got, tc.want) {
				t.Fatalf("uncensored codes = %v, want %v", got, tc.want)
			}
		})
	}
	for _, filename := range []string{"FC2-PPV-.mp4", "FC2-PPV-abc.mp4", "FC2-PPV-１２３.mp4", "XFC2PPV1234567.mp4"} {
		for _, code := range util.ExtractCodeFromName(filename) {
			if strings.HasPrefix(code, "FC2-PPV-") {
				t.Fatalf("invalid FC2 filename %q produced code %q", filename, code)
			}
		}
	}
}
