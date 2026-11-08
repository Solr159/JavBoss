package subtitle

import "testing"

func TestFileName(t *testing.T) {
	for _, tc := range []struct{ code, language, want string }{
		{"abc-123", "zh-cn", "ABC-123.zh-CN.srt"},
		{"FC2-PPV-1234567", "zh-TW", "FC2-PPV-1234567.zh-TW.srt"},
		{"123456_123", "ja", "123456_123.ja.srt"},
		{"ABC-123", "zh-hant", "ABC-123.zh-Hant.srt"},
		{"../ABC-123", "zh-CN", ""}, {`ABC\123`, "zh-CN", ""},
		{"ABC:123", "zh-CN", ""}, {"ABC-123", "../zh-CN", ""}, {"", "zh-CN", ""},
	} {
		t.Run(tc.code+"/"+tc.language, func(t *testing.T) {
			got, err := FileName(tc.code, tc.language)
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatalf("got=%q err=%v want=%q", got, err, tc.want)
			}
		})
	}
}
