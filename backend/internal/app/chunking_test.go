package app

import (
	"strings"
	"testing"
)

func TestAutoChunkingUnicodeOverlapAndBoundary(t *testing.T) {
	text := strings.Repeat("甲", 390) + "。" + strings.Repeat("乙", 500)
	chunks, err := SplitKnowledge(text, ChunkOptions{Strategy: "auto"})
	if err != nil || len(chunks) < 2 {
		t.Fatalf("chunks=%d err=%v", len(chunks), err)
	}
	for index, chunk := range chunks {
		if len([]rune(chunk.Text)) > 800 || chunk.Index != index || chunk.EndOffset <= chunk.StartOffset {
			t.Fatalf("invalid chunk: %+v", chunk)
		}
	}
	if chunks[1].StartOffset != chunks[0].EndOffset-80 {
		t.Fatalf("overlap=%d", chunks[0].EndOffset-chunks[1].StartOffset)
	}
}

func TestAutoChunkingLongUnbrokenText(t *testing.T) {
	chunks, _ := SplitKnowledge(strings.Repeat("界", 1700), ChunkOptions{})
	if len(chunks) != 3 || len([]rune(chunks[0].Text)) != 800 {
		t.Fatalf("unexpected chunks: %+v", chunks)
	}
}

func TestCustomChunkingValidationAndPreprocessing(t *testing.T) {
	_, err := SplitKnowledge("text", ChunkOptions{Strategy: "custom", MaxLength: 99, Separator: "newline"})
	if err == nil {
		t.Fatal("expected validation error")
	}
	text := "第一段  https://example.com\n\n联系 test@example.com   第二段" + strings.Repeat("文", 120)
	chunks, err := SplitKnowledge(text, ChunkOptions{Strategy: "custom", MaxLength: 100, OverlapPercent: 20, Separator: "blank-line", RemoveURLsEmails: true, CollapseWhitespace: true})
	if err != nil || len(chunks) == 0 {
		t.Fatalf("chunks=%d err=%v", len(chunks), err)
	}
	if strings.Contains(chunks[0].Text, "http") || strings.Contains(chunks[0].Text, "@") || strings.Contains(chunks[0].Text, "  ") {
		t.Fatalf("preprocessing failed: %q", chunks[0].Text)
	}
}

func TestHierarchyChunkingRetainsHeadings(t *testing.T) {
	text := "前言\n# 第一章\n内容\n## 小节\n" + strings.Repeat("长", 900) + "\n# 第二章\n结尾"
	chunks, err := SplitKnowledge(text, ChunkOptions{Strategy: "hierarchy"})
	if err != nil || len(chunks) < 4 {
		t.Fatalf("chunks=%d err=%v", len(chunks), err)
	}
	joined := ""
	for _, chunk := range chunks {
		joined += chunk.Text
	}
	if !strings.Contains(joined, "# 第一章") || !strings.Contains(joined, "## 小节") || !strings.Contains(joined, "# 第二章") {
		t.Fatal("headings were not retained")
	}
}
