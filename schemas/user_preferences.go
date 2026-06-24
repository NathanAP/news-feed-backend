package schemas

type Theme string

const (
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"
)

func (t Theme) IsValid() bool {
	return t == ThemeLight || t == ThemeDark
}

type Language string

const (
	LanguagePT Language = "pt"
	LanguageEN Language = "en"
	LanguageES Language = "es"
	LanguageFR Language = "fr"
	LanguageDE Language = "de"
	LanguageIT Language = "it"
	LanguageJA Language = "ja"
	LanguageZH Language = "zh"
)

func (l Language) IsValid() bool {
	switch l {
	case LanguagePT, LanguageEN, LanguageES, LanguageFR,
		LanguageDE, LanguageIT, LanguageJA, LanguageZH:
		return true
	}
	return false
}

type AIPersonality string

const (
	AIPersonalityFun         AIPersonality = "fun"
	AIPersonalityInformative AIPersonality = "informative"
	AIPersonalityMixed       AIPersonality = "mixed"
)

func (p AIPersonality) IsValid() bool {
	return p == AIPersonalityFun || p == AIPersonalityInformative || p == AIPersonalityMixed
}

type UserPreferencesResponse struct {
	Theme            Theme         `json:"theme"`
	Language         Language      `json:"language"`
	TranslateContent bool          `json:"translate_content"`
	AIPersonality    AIPersonality `json:"ai_personality"`
}

type UpdateUserPreferencesRequest struct {
	Theme            Theme         `json:"theme"`
	Language         Language      `json:"language"`
	TranslateContent bool          `json:"translate_content"`
	AIPersonality    AIPersonality `json:"ai_personality"`
}
