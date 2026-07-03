package enums

// Language is the application-wide set of supported languages. It is the single source of truth
// shared by every feature that reasons about language: user preferences (the reader's language),
// articles (their detected language_original), and translation (the target language). It must stay
// aligned with the languages configured for detection in services/langdetect (CLAUDE.md).
type Language string

const (
	LanguagePT Language = "pt"
	LanguageEN Language = "en"
	LanguageES Language = "es"
	LanguageFR Language = "fr"
	LanguageDE Language = "de"
	LanguageIT Language = "it"
)

func (l Language) IsValid() bool {
	switch l {
	case LanguagePT, LanguageEN, LanguageES, LanguageFR,
		LanguageDE, LanguageIT:
		return true
	}
	return false
}

// DisplayName returns the English name of the language, used to instruct the AI which language to
// translate into. Returns an empty string for an unknown code.
func (l Language) DisplayName() string {
	switch l {
	case LanguagePT:
		return "Portuguese"
	case LanguageEN:
		return "English"
	case LanguageES:
		return "Spanish"
	case LanguageFR:
		return "French"
	case LanguageDE:
		return "German"
	case LanguageIT:
		return "Italian"
	}
	return ""
}
