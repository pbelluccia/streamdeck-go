package app

import (
	"context"
	"reflect"
	"testing"
)

func TestMediaPlayerUsesConfiguredPlayer(t *testing.T) {
	player := NewMediaPlayer("vlc")

	args, ok := player.controlArgs("play_pause")
	if !ok {
		t.Fatal("play_pause command was not supported")
	}
	want := []string{"--player=vlc", "play-pause"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("controlArgs(play_pause) = %v, want %v", args, want)
	}
}

func TestMediaPlayerSupportsMediaCommands(t *testing.T) {
	tests := map[string]string{
		"previous":   "previous",
		"play_pause": "play-pause",
		"play-pause": "play-pause",
		"next":       "next",
		"play":       "play",
		"pause":      "pause",
		"stop":       "stop",
	}

	for command, want := range tests {
		got, ok := mapMediaCommand(command)
		if !ok {
			t.Fatalf("mapMediaCommand(%q) was not supported", command)
		}
		if got != want {
			t.Fatalf("mapMediaCommand(%q) = %q, want %q", command, got, want)
		}
	}
}

func TestParseMediaLine(t *testing.T) {
	state := parseMediaLine("Playing|file:///tmp/cover.png")

	if state.Status != "Playing" {
		t.Fatalf("Status = %q, want Playing", state.Status)
	}
	if state.ArtURL != "file:///tmp/cover.png" {
		t.Fatalf("ArtURL = %q, want file:///tmp/cover.png", state.ArtURL)
	}
}

func TestAlbumArtEmptyURLReturnsNil(t *testing.T) {
	player := NewMediaPlayer("vlc")

	if got := player.AlbumArt(context.Background(), ""); got != nil {
		t.Fatalf("AlbumArt(empty) = %v, want nil", got)
	}
}
