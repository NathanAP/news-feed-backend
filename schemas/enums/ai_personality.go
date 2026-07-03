package enums

type AIPersonality string

const (
	AIPersonalityFun         AIPersonality = "fun"
	AIPersonalityInformative AIPersonality = "informative"
	AIPersonalityMixed       AIPersonality = "mixed"
)

func (p AIPersonality) IsValid() bool {
	return p == AIPersonalityFun || p == AIPersonalityInformative || p == AIPersonalityMixed
}
