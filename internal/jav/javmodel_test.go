package jav

import (
	"errors"
	"testing"
)

func TestFinalizeJavModelActressInfoRequiresParsedMatchingJapaneseName(t *testing.T) {
	t.Run("accepts matching parsed japanese name", func(t *testing.T) {
		info, err := finalizeJavModelActressInfo("小橋りえこ", "Rieko Kobashi", "https://javmodel.com/jav/Rieko-Kobashi", &ActressInfo{
			JapaneseName: " 小橋りえこ ",
		})
		if err != nil {
			t.Fatalf("finalizeJavModelActressInfo: %v", err)
		}
		if info.JapaneseName != "小橋りえこ" {
			t.Fatalf("unexpected japanese name: %q", info.JapaneseName)
		}
		if info.RomanName != "Rieko Kobashi" {
			t.Fatalf("unexpected roman name: %q", info.RomanName)
		}
		if info.ProfileURL != "https://javmodel.com/jav/Rieko-Kobashi" {
			t.Fatalf("unexpected profile url: %q", info.ProfileURL)
		}
	})

	t.Run("rejects missing parsed japanese name", func(t *testing.T) {
		info, err := finalizeJavModelActressInfo("小橋りえこ", "Rieko Kobashi", "https://javmodel.com/jav/Rieko-Kobashi", &ActressInfo{})
		if !errors.Is(err, ResourceNotFonud) {
			t.Fatalf("err = %v, want ResourceNotFonud", err)
		}
		if info != nil {
			t.Fatalf("info = %#v, want nil", info)
		}
	})

	t.Run("rejects mismatched parsed japanese name", func(t *testing.T) {
		info, err := finalizeJavModelActressInfo("小橋りえこ", "Rieko Kobashi", "https://javmodel.com/jav/Rieko-Kobashi", &ActressInfo{
			JapaneseName: "別の女優",
		})
		if !errors.Is(err, ResourceNotFonud) {
			t.Fatalf("err = %v, want ResourceNotFonud", err)
		}
		if info != nil {
			t.Fatalf("info = %#v, want nil", info)
		}
	})
}
