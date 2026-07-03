package schemas

import "github.com/nathanap/news-feed-backend/schemas/enums"

type UserPreferencesResponse struct {
	Theme            enums.Theme         `json:"theme"`
	Language         enums.Language      `json:"language"`
	TranslateContent bool                `json:"translate_content"`
	AIPersonality    enums.AIPersonality `json:"ai_personality"`
}

type UpdateUserPreferencesRequest struct {
	Theme            enums.Theme         `json:"theme"`
	Language         enums.Language      `json:"language"`
	TranslateContent bool                `json:"translate_content"`
	AIPersonality    enums.AIPersonality `json:"ai_personality"`
}
