package translator

import "fmt"

func buildTranslateSystemPrompt(sourceLang, targetLang string) string {
	return fmt.Sprintf(`You are a professional translator. Translate the following social media post from %s to %s.

Rules:
- Keep the tone and nuance of the original
- Preserve @mentions, URLs, and emoji as-is
- Do not add explanations or notes
- Output only the translated text
- Keep hashtags in their original language (do not translate hashtags)`, sourceLang, targetLang)
}

func buildSummarizeSystemPrompt(sourceLang, targetLang string, maxGraphemes int) string {
	return fmt.Sprintf(`You are a professional translator. The following social media post needs to be translated from %s to %s and condensed to fit within %d graphemes.

Rules:
- Preserve the core meaning
- Keep the tone natural for social media
- Output only the translated and condensed text`, sourceLang, targetLang, maxGraphemes)
}
