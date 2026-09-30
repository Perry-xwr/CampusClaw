package app

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type ChunkOptions struct {
	Strategy           string `json:"strategy"`
	MaxLength          int    `json:"max_length,omitempty"`
	OverlapPercent     int    `json:"overlap_percent,omitempty"`
	Separator          string `json:"separator,omitempty"`
	RemoveURLsEmails   bool   `json:"remove_urls_emails,omitempty"`
	CollapseWhitespace bool   `json:"collapse_whitespace,omitempty"`
}

type ChunkDraft struct {
	Index       int
	Text        string
	StartOffset int
	EndOffset   int
}

func SplitKnowledge(text string, options ChunkOptions) ([]ChunkDraft, error) {
	strategy := options.Strategy
	if strategy == "" {
		strategy = "auto"
	}
	switch strategy {
	case "auto":
		return splitWindow(text, 800, 80, []string{"\n\n", "\n", "。"}), nil
	case "custom":
		if options.MaxLength < 100 || options.MaxLength > 2000 {
			return nil, fmt.Errorf("custom max_length must be between 100 and 2000")
		}
		if options.OverlapPercent < 0 || options.OverlapPercent > 50 {
			return nil, fmt.Errorf("custom overlap_percent must be between 0 and 50")
		}
		separator := map[string]string{"newline": "\n", "blank-line": "\n\n", "period": "。"}[options.Separator]
		if separator == "" {
			return nil, fmt.Errorf("custom separator must be newline, blank-line, or period")
		}
		processed := text
		if options.RemoveURLsEmails {
			processed = regexp.MustCompile(`https?://\S+|[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}`).ReplaceAllString(processed, "")
		}
		if options.CollapseWhitespace {
			processed = strings.Join(strings.Fields(processed), " ")
		}
		overlap := options.MaxLength * options.OverlapPercent / 100
		return splitWindow(processed, options.MaxLength, overlap, []string{separator}), nil
	case "hierarchy":
		return splitHierarchy(text), nil
	default:
		return nil, fmt.Errorf("strategy must be auto, custom, or hierarchy")
	}
}

func splitWindow(text string, maxLength, overlap int, boundaries []string) []ChunkDraft {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	result := []ChunkDraft{}
	for start := 0; start < len(runes); {
		end := start + maxLength
		if end >= len(runes) {
			end = len(runes)
		} else {
			minimum := start + maxLength/2
			best := -1
			for _, boundary := range boundaries {
				needle := []rune(boundary)
				for pos := end - len(needle); pos >= minimum; pos-- {
					if string(runes[pos:pos+len(needle)]) == boundary {
						candidate := pos + len(needle)
						if candidate > best {
							best = candidate
						}
						break
					}
				}
			}
			if best > start {
				end = best
			}
		}
		result = append(result, ChunkDraft{Index: len(result), Text: string(runes[start:end]), StartOffset: start, EndOffset: end})
		if end == len(runes) {
			break
		}
		next := end - overlap
		if next <= start {
			next = end
		}
		start = next
	}
	return result
}

func splitHierarchy(text string) []ChunkDraft {
	runes := []rune(text)
	starts := []int{0}
	for index, r := range runes {
		if index > 0 && runes[index-1] != '\n' {
			continue
		}
		if r != '#' {
			continue
		}
		level := 0
		for index+level < len(runes) && runes[index+level] == '#' {
			level++
		}
		if level >= 1 && level <= 3 && index+level < len(runes) && unicode.IsSpace(runes[index+level]) && index != 0 {
			starts = append(starts, index)
		}
	}
	starts = append(starts, len(runes))
	result := []ChunkDraft{}
	for index := 0; index < len(starts)-1; index++ {
		start, end := starts[index], starts[index+1]
		if start == end {
			continue
		}
		section := string(runes[start:end])
		parts := splitWindow(section, 800, 80, []string{"\n\n", "\n", "。"})
		for _, part := range parts {
			part.Index = len(result)
			part.StartOffset += start
			part.EndOffset += start
			result = append(result, part)
		}
	}
	return result
}
