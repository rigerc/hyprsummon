package cmd

import (
	"fmt"
	"os"
)

const (
	ansiReset = "\033[0m"
	ansiBold  = "\033[1m"
	ansiDim   = "\033[2m"
	ansiCyan  = "\033[36m"
	ansiGreen = "\033[32m"
)

func styleBold(text string) string {
	return applyStyle(ansiBold, text)
}

func styleDim(text string) string {
	return applyStyle(ansiDim, text)
}

func styleLabel(text string) string {
	return applyStyle(ansiBold+ansiCyan, text)
}

func styleValue(text string) string {
	return applyStyle(ansiGreen, text)
}

func applyStyle(prefix string, text string) string {
	if text == "" || os.Getenv("NO_COLOR") != "" {
		return text
	}
	return fmt.Sprintf("%s%s%s", prefix, text, ansiReset)
}
