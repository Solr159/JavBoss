package translation

import "strings"

// Config keys shared by the title translation pipeline.
const (
	// LanguageKey selects the display and translation target language ("zh" or "en").
	LanguageKey = "jav_title_language"
	// PromptKey holds the Chinese translation prompt; PromptKeyEN holds the English one.
	PromptKey   = "jav_title_translation_prompt"
	PromptKeyEN = "jav_title_translation_prompt_en"
	// DefaultPromptKey and DefaultPromptKeyEN are read-only defaults exposed to the UI.
	DefaultPromptKey   = "jav_title_translation_default_prompt"
	DefaultPromptKeyEN = "jav_title_translation_default_prompt_en"
)

const DefaultPrompt = `将日文影视标题翻译成准确、自然、通顺的简体中文。

要求：
1. 充分考虑中日文的表达差异，避免逐词直译。惯用语结合上下文意译，对白自然口语化。
2. 日文长句可适当调整语序、拆分短句、补充中文标点，避免冗长的“……的”修饰结构，保留原有逻辑关系。
3. 准确理解人称、人物关系和主被动；不增添情节，不为使情节合理而改写事实。
4. 人名和番号保持原样，不改动姓名中的假名或汉字，尤其注意末尾演员姓名。NTR 等类型标签可保留。
5. 忠实保留原意和语气，不扩写、不夸张。输出前检查中文语序与搭配。

只输出译文，不附解释或包裹整个标题的引号。`

const DefaultPromptEN = `Translate Japanese adult-video titles into accurate, natural English.

Requirements:
1. Translate for meaning and idiom, not word by word. Keep the phrasing natural for an English-speaking viewer.
2. Restructure long Japanese sentences as needed; prefer clear, readable English over literal word order.
3. Understand characters, relationships and voice; never invent plot details or alter facts to make a sentence read better.
4. Keep performer names and product codes unchanged, including names at the end of the title. Genre tags such as NTR may stay as-is.
5. Stay faithful to the original meaning and tone; do not embellish.

Output only the translated title, with no explanation or surrounding quotes.`

// TargetLanguage resolves the configured translation target language ("zh" or "en").
func TargetLanguage(cfg map[string]string) string {
	if strings.EqualFold(strings.TrimSpace(cfg[LanguageKey]), "en") {
		return "en"
	}
	return "zh"
}

// PromptFor returns the prompt matching the configured target language, falling
// back to that language's default when no custom prompt is saved.
func PromptFor(cfg map[string]string) string {
	if TargetLanguage(cfg) == "en" {
		if prompt := strings.TrimSpace(cfg[PromptKeyEN]); prompt != "" {
			return prompt
		}
		return DefaultPromptEN
	}
	if prompt := strings.TrimSpace(cfg[PromptKey]); prompt != "" {
		return prompt
	}
	return DefaultPrompt
}
