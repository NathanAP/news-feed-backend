package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/nathanap/news-feed-backend/logger"
)

// NewVerboseTreater wraps a Treater so each call logs a START/END marker with elapsed time. Gated
// by its own flag (TREATMENT_VERBOSE_MODE) so treatment can be traced independently of keywords.
// When disabled it returns the inner treater unchanged.
func NewVerboseTreater(inner Treater, label string, enabled bool) Treater {
	if !enabled {
		return inner
	}
	return verboseTreater{inner: inner, label: label}
}

type verboseTreater struct {
	inner Treater
	label string
}

func (v verboseTreater) Treat(ctx context.Context, title, content string) (string, error) {
	logger.Print(fmt.Sprintf("@@@ TREATMENT START - %s @@@", v.label), logger.ColorCyan)
	start := time.Now()

	out, err := v.inner.Treat(ctx, title, content)

	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		logger.Print(fmt.Sprintf("@@@ TREATMENT FAILED - %s - %s: %v @@@", v.label, elapsed, err), logger.ColorRed)
	} else {
		logger.Print(fmt.Sprintf("@@@ TREATMENT END - %s - %s (%d chars) @@@", v.label, elapsed, len(out)), logger.ColorGreen)
	}
	return out, err
}

// NewVerboseKeyworder wraps a Keyworder so each call logs a START/END marker with elapsed time.
// Gated by its own flag (KEYWORDS_VERBOSE_MODE). When disabled it returns the inner keyworder.
func NewVerboseKeyworder(inner Keyworder, label string, enabled bool) Keyworder {
	if !enabled {
		return inner
	}
	return verboseKeyworder{inner: inner, label: label}
}

type verboseKeyworder struct {
	inner Keyworder
	label string
}

func (v verboseKeyworder) Keywords(ctx context.Context, title, content string) ([]string, error) {
	logger.Print(fmt.Sprintf("@@@ KEYWORDS START - %s @@@", v.label), logger.ColorCyan)
	start := time.Now()

	out, err := v.inner.Keywords(ctx, title, content)

	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		logger.Print(fmt.Sprintf("@@@ KEYWORDS FAILED - %s - %s: %v @@@", v.label, elapsed, err), logger.ColorRed)
	} else {
		logger.Print(fmt.Sprintf("@@@ KEYWORDS END - %s - %s (%d keywords) @@@", v.label, elapsed, len(out)), logger.ColorGreen)
	}
	return out, err
}

// NewVerboseJudger wraps a Judger so each call logs a START/END marker with elapsed time and the
// resulting score. Gated by its own flag (JUDGEMENT_VERBOSE_MODE). When disabled it returns the
// inner judger unchanged.
func NewVerboseJudger(inner Judger, label string, enabled bool) Judger {
	if !enabled {
		return inner
	}
	return verboseJudger{inner: inner, label: label}
}

type verboseJudger struct {
	inner Judger
	label string
}

func (v verboseJudger) Judge(ctx context.Context, feedKeywords []string, title string, articleKeywords []string, content string) (int, error) {
	logger.Print(fmt.Sprintf("@@@ JUDGEMENT START - %s @@@", v.label), logger.ColorCyan)
	start := time.Now()

	score, err := v.inner.Judge(ctx, feedKeywords, title, articleKeywords, content)

	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		logger.Print(fmt.Sprintf("@@@ JUDGEMENT FAILED - %s - %s: %v @@@", v.label, elapsed, err), logger.ColorRed)
	} else {
		logger.Print(fmt.Sprintf("@@@ JUDGEMENT END - %s - %s (score %d) @@@", v.label, elapsed, score), logger.ColorGreen)
	}
	return score, err
}

// NewVerboseTranslator wraps a Translator so each call logs a START/END marker with elapsed time.
// Gated by its own flag (TRANSLATION_VERBOSE_MODE). When disabled it returns the inner translator.
func NewVerboseTranslator(inner Translator, label string, enabled bool) Translator {
	if !enabled {
		return inner
	}
	return verboseTranslator{inner: inner, label: label}
}

type verboseTranslator struct {
	inner Translator
	label string
}

func (v verboseTranslator) Translate(ctx context.Context, targetLanguage, personality, title, content string) (Translation, error) {
	logger.Print(fmt.Sprintf("@@@ TRANSLATION START - %s -> %s @@@", v.label, targetLanguage), logger.ColorCyan)
	start := time.Now()

	out, err := v.inner.Translate(ctx, targetLanguage, personality, title, content)

	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		logger.Print(fmt.Sprintf("@@@ TRANSLATION FAILED - %s - %s: %v @@@", v.label, elapsed, err), logger.ColorRed)
	} else {
		logger.Print(fmt.Sprintf("@@@ TRANSLATION END - %s - %s (%d chars) @@@", v.label, elapsed, len(out.Content)), logger.ColorGreen)
	}
	return out, err
}
