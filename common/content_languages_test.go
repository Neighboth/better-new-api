package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeContentLanguage(t *testing.T) {
	cases := map[string]string{
		"tr":      "tr",
		"TR":      "tr",
		"zh-cn":   "zh-CN",
		"zh_CN":   "zh-CN",
		"zh-tw":   "zh-TW",
		"en":      "en",
		"xx":      "",
		"":        "",
		"klingon": "",
	}
	for in, want := range cases {
		assert.Equal(t, want, NormalizeContentLanguage(in), "input: %q", in)
	}
}

func TestContentLanguagesContainEnglishFirst(t *testing.T) {
	require := assert.New(t)
	require.Equal(DefaultContentLanguage, ContentLanguages[0].Code)
	seen := map[string]bool{}
	for _, lang := range ContentLanguages {
		require.False(seen[lang.Code], "duplicate language code %s", lang.Code)
		seen[lang.Code] = true
		require.NotEmpty(lang.Native)
		require.NotEmpty(lang.Flag)
	}
}
