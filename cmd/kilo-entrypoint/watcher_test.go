package main

import (
	"encoding/binary"
	"testing"
)

// buildInotifyEvent constructs a raw inotify event byte slice.
func buildInotifyEvent(wd int32, mask, cookie, nameLen uint32, name string) []byte {
	// 16 bytes header + name + padding to 4-byte boundary
	total := 16 + int(nameLen)
	for total%4 != 0 {
		total++
	}
	buf := make([]byte, total)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(wd))
	binary.LittleEndian.PutUint32(buf[4:8], mask)
	binary.LittleEndian.PutUint32(buf[8:12], cookie)
	binary.LittleEndian.PutUint32(buf[12:16], nameLen)
	if nameLen > 0 {
		copy(buf[16:], name)
	}
	return buf
}

func TestParseInotifyEventsSingle(t *testing.T) {
	buf := buildInotifyEvent(1, 0x00000002, 100, uint32(len("test.md")+1), "test.md")
	events := parseInotifyEvents(buf, len(buf))
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.wd != 1 {
		t.Errorf("wd = %d, want 1", ev.wd)
	}
	if ev.mask != 0x00000002 {
		t.Errorf("mask = %d, want 2", ev.mask)
	}
	if ev.cookie != 100 {
		t.Errorf("cookie = %d, want 100", ev.cookie)
	}
	if ev.name != "test.md" {
		t.Errorf("name = %q, want %q", ev.name, "test.md")
	}
}

func TestParseInotifyEventsMultiple(t *testing.T) {
	e1 := buildInotifyEvent(1, 0x00000001, 0, uint32(len("a.md")+1), "a.md")
	e2 := buildInotifyEvent(2, 0x00000004, 0, uint32(len("b.md")+1), "b.md")
	buf := append(e1, e2...)

	events := parseInotifyEvents(buf, len(buf))
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].name != "a.md" {
		t.Errorf("event 0 name = %q, want a.md", events[0].name)
	}
	if events[1].name != "b.md" {
		t.Errorf("event 1 name = %q, want b.md", events[1].name)
	}
}

func TestParseInotifyEventsNoName(t *testing.T) {
	buf := buildInotifyEvent(3, 0x00000002, 0, 0, "")
	events := parseInotifyEvents(buf, len(buf))
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].name != "" {
		t.Errorf("name = %q, want empty", events[0].name)
	}
	if events[0].wd != 3 {
		t.Errorf("wd = %d, want 3", events[0].wd)
	}
}

func TestParseInotifyEventsEmpty(t *testing.T) {
	events := parseInotifyEvents([]byte{}, 0)
	if len(events) != 0 {
		t.Errorf("expected 0 events for empty buffer, got %d", len(events))
	}
}

func TestParseInotifyEventsTruncatedHeader(t *testing.T) {
	// Less than 16 bytes: should return no events
	buf := []byte{1, 2, 3, 4}
	events := parseInotifyEvents(buf, len(buf))
	if len(events) != 0 {
		t.Errorf("expected 0 events for truncated header, got %d", len(events))
	}
}

func TestParseInotifyEventsZeroLength(t *testing.T) {
	buf := buildInotifyEvent(1, 0x00000002, 0, uint32(len("test.md")+1), "test.md")
	// Pass n=0 to simulate empty read
	events := parseInotifyEvents(buf, 0)
	if len(events) != 0 {
		t.Errorf("expected 0 events for n=0, got %d", len(events))
	}
}

func TestParseInotifyEventsAlignment(t *testing.T) {
	// Name length that doesn't align to 4 bytes
	// "ab" has len 3 (with null), 16+3 = 19, padded to 20
	e1 := buildInotifyEvent(1, 0x00000002, 0, uint32(len("ab")+1), "ab")
	e2 := buildInotifyEvent(2, 0x00000001, 0, uint32(len("cd")+1), "cd")
	buf := append(e1, e2...)

	events := parseInotifyEvents(buf, len(buf))
	if len(events) != 2 {
		t.Fatalf("expected 2 events with alignment, got %d", len(events))
	}
	if events[0].name != "ab" {
		t.Errorf("event 0 name = %q, want ab", events[0].name)
	}
	if events[1].name != "cd" {
		t.Errorf("event 1 name = %q, want cd", events[1].name)
	}
}

func TestParseInotifyEventsLongName(t *testing.T) {
	longName := "very-long-filename-that-exceeds-normal-length.md"
	buf := buildInotifyEvent(1, 0x00000002, 42, uint32(len(longName)+1), longName)
	events := parseInotifyEvents(buf, len(buf))
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].name != longName {
		t.Errorf("name = %q, want %q", events[0].name, longName)
	}
	if events[0].cookie != 42 {
		t.Errorf("cookie = %d, want 42", events[0].cookie)
	}
}
