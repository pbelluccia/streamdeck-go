package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"streamdeck-go/internal/app"
	"streamdeck-go/internal/deck"
)

func main() {
	cfg, err := app.LoadConfig()
	if err != nil {
		fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runDeviceLoop(ctx, cfg)
}

func runDeviceLoop(ctx context.Context, cfg app.Config) {
	hidWakeup := newHIDWakeup(ctx)
	defer hidWakeup.Close()

	backoff := 2 * time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		deckDevice, err := deck.Open(cfg.DevicePath, cfg.DeviceModel)
		if err != nil {
			fmt.Printf("StreamDeck unavailable: %v; retrying within %s\n", err, backoff)
			sleepUntilDeviceRetry(ctx, backoff, hidWakeup.C())
			backoff = nextBackoff(backoff)
			continue
		}

		backoff = 2 * time.Second
		fmt.Printf("StreamDeck connected: %s (%s)\n", deckDevice.Path(), deckDevice.Model())
		fmt.Println("Config loaded page:", cfg.StartPage)

		err = app.New(deckDevice, cfg).Run(ctx)
		if closeErr := deckDevice.Close(); closeErr != nil {
			fmt.Println("Error closing StreamDeck:", closeErr)
		}
		if errors.Is(err, context.Canceled) {
			return
		}
		fmt.Printf("StreamDeck disconnected or failed: %v; reconnecting within %s\n", err, backoff)
		sleepUntilDeviceRetry(ctx, backoff, hidWakeup.C())
		backoff = nextBackoff(backoff)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func sleepUntilDeviceRetry(ctx context.Context, duration time.Duration, wakeups <-chan struct{}) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	case <-wakeups:
	}
}

func nextBackoff(current time.Duration) time.Duration {
	current *= 2
	if current > time.Minute {
		return time.Minute
	}
	return current
}
