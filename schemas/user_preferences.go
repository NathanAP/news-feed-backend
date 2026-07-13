package schemas

import "github.com/nathanap/news-feed-backend/schemas/enums"

type UserPreferencesResponse struct {
	// LanguageToTranslate is null when the user has no translation target set (the client then
	// hides the translation option). It is a client hint only and does not gate any API behavior.
	LanguageToTranslate *enums.Language     `json:"language_to_translate"`
	AIPersonality       enums.AIPersonality `json:"ai_personality"`
}

type UpdateUserPreferencesRequest struct {
	// LanguageToTranslate omitted/null clears the translation target; when present it must be a
	// valid language enum.
	LanguageToTranslate *enums.Language     `json:"language_to_translate"`
	AIPersonality       enums.AIPersonality `json:"ai_personality"`
}
