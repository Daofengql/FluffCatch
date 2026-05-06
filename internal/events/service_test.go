package events

import "testing"

func TestNormalizeSlugAllowsChineseAndCleansSeparators(t *testing.T) {
	got := normalizeSlug("  测试 活动!! 2026  ")
	if got != "测试-活动-2026" {
		t.Fatalf("expected cleaned slug, got %q", got)
	}
}

func TestNormalizeSlugTrimsLongValues(t *testing.T) {
	got := normalizeSlug("abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz")
	if len([]rune(got)) > 80 {
		t.Fatalf("expected slug length <= 80, got %d", len([]rune(got)))
	}
}

func TestFriendlySQLErrorForDuplicateSlug(t *testing.T) {
	err := friendlySQLError("create event", assertError("Error 1062 (23000): Duplicate entry '测试' for key 'events_slug_unique'"))
	if err.Error() != "create event: slug already exists; please use a different URL identifier" {
		t.Fatalf("unexpected error: %v", err)
	}
}

type assertError string

func (err assertError) Error() string {
	return string(err)
}
