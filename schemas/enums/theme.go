// Package enums holds the application's shared enumerations, one type per file. Keeping them in a
// dedicated, dependency-free package lets any layer reference them as `enums.X` without pulling in
// the larger `schemas` package (DTOs) or risking import cycles.
package enums

type Theme string

const (
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"
)

func (t Theme) IsValid() bool {
	return t == ThemeLight || t == ThemeDark
}
