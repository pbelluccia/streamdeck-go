package app

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"streamdeck-go/internal/render"
)

type MediaPlayer struct {
	player     string
	lastArtURL string
	lastArt    image.Image
	client     *http.Client
}

type MediaState struct {
	Status string
	ArtURL string
}

func NewMediaPlayer(player string) *MediaPlayer {
	player = strings.TrimSpace(player)
	return &MediaPlayer{
		player: player,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (m *MediaPlayer) Player() string {
	return m.player
}

func (m *MediaPlayer) Status(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "playerctl", m.playerArg(), "status").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (m *MediaPlayer) AlbumArt(ctx context.Context, url string) image.Image {
	if url == "" {
		m.lastArtURL = ""
		m.lastArt = nil
		return nil
	}
	if url == m.lastArtURL && m.lastArt != nil {
		return m.lastArt
	}

	var (
		data []byte
		err  error
	)
	if strings.HasPrefix(url, "file://") {
		path := strings.TrimPrefix(url, "file://")
		data, err = os.ReadFile(path)
	} else {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if reqErr != nil {
			fmt.Println("Error creating album art request:", reqErr)
			return nil
		}
		resp, reqErr := m.client.Do(req)
		if reqErr != nil {
			fmt.Println("Error downloading album art:", reqErr)
			return nil
		}
		defer resp.Body.Close()
		buf := &bytes.Buffer{}
		_, err = buf.ReadFrom(resp.Body)
		data = buf.Bytes()
	}
	if err != nil {
		fmt.Println("Error loading album art image:", err)
		return nil
	}
	img, err := render.DecodeImage(data)
	if err != nil {
		fmt.Println("Error decoding album art image:", err)
		return nil
	}
	m.lastArtURL = url
	m.lastArt = img
	return img
}

func (m *MediaPlayer) InitialState(ctx context.Context) MediaState {
	out, err := exec.CommandContext(ctx, "playerctl", m.playerArg(), "metadata", "--format", "{{status}}|{{mpris:artUrl}}").Output()
	if err != nil {
		return MediaState{}
	}
	return parseMediaLine(string(out))
}

func (m *MediaPlayer) Watch(ctx context.Context, updates chan<- MediaState) {
	backoff := 2 * time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		cmd := exec.CommandContext(ctx, "playerctl", m.playerArg(), "--follow", "metadata", "--format", "{{status}}|{{mpris:artUrl}}")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			sleepContext(ctx, backoff)
			backoff = nextBackoff(backoff)
			continue
		}
		stderr, _ := cmd.StderrPipe()
		if err := cmd.Start(); err != nil {
			sleepContext(ctx, backoff)
			backoff = nextBackoff(backoff)
			continue
		}
		go io.Copy(io.Discard, stderr)
		backoff = 2 * time.Second

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			state := parseMediaLine(scanner.Text())
			select {
			case updates <- state:
			case <-ctx.Done():
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				return
			}
		}
		_ = cmd.Wait()
		sleepContext(ctx, backoff)
		backoff = nextBackoff(backoff)
	}
}

func (m *MediaPlayer) Control(ctx context.Context, command string) {
	args, ok := m.controlArgs(command)
	if !ok {
		fmt.Println("Unsupported media command:", command)
		return
	}
	runCommand(ctx, "playerctl", args...)
}

func (m *MediaPlayer) controlArgs(command string) ([]string, bool) {
	playerctlCommand, ok := mapMediaCommand(command)
	if !ok {
		return nil, false
	}
	return []string{m.playerArg(), playerctlCommand}, true
}

func (m *MediaPlayer) playerArg() string {
	return "--player=" + m.player
}

func canonicalPlayer(player, fallback string) string {
	player = strings.TrimSpace(player)
	if player != "" {
		return player
	}
	return strings.TrimSpace(fallback)
}

func mapMediaCommand(command string) (string, bool) {
	switch strings.TrimSpace(command) {
	case "previous":
		return "previous", true
	case "play_pause", "play-pause":
		return "play-pause", true
	case "next":
		return "next", true
	case "play":
		return "play", true
	case "pause":
		return "pause", true
	case "stop":
		return "stop", true
	default:
		return "", false
	}
}

func parseMediaLine(line string) MediaState {
	line = strings.TrimSpace(line)
	if line == "" {
		return MediaState{}
	}
	parts := strings.SplitN(line, "|", 2)
	state := MediaState{Status: strings.TrimSpace(parts[0])}
	if len(parts) > 1 {
		state.ArtURL = strings.TrimSpace(parts[1])
	}
	return state
}

func sleepContext(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

func nextBackoff(current time.Duration) time.Duration {
	current *= 2
	if current > time.Minute {
		return time.Minute
	}
	return current
}

func runCommand(ctx context.Context, name string, args ...string) {
	if err := exec.CommandContext(ctx, name, args...).Run(); err != nil {
		fmt.Println("Command error:", name, strings.Join(args, " "), err)
	}
}

func runShellCommand(ctx context.Context, command string) {
	command = strings.TrimSpace(command)
	if command == "" {
		return
	}
	runCommand(ctx, "sh", "-c", command)
}
