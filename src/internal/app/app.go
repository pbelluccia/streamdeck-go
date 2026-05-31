package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"sync"
	"time"

	"streamdeck-go/internal/deck"
	"streamdeck-go/internal/render"
)

type Config struct {
	DevicePath     string
	DeviceModel    string
	IconDir        string
	FontPath       string
	Location       string
	MediaPlayer    string
	Brightness     byte
	HoldMS         int
	WeatherRefresh time.Duration
	StartPage      string
	Pages          map[string]Page
}

type App struct {
	deck         *deck.Device
	renderer     *render.Renderer
	media        *MediaPlayer
	mediaPlayers map[string]*MediaPlayer
	weather      *WeatherClient
	config       Config
	layout       render.Layout
	currentPage  string
	mode         int
	brightness   byte
	mediaState   MediaState
	keyStates    []keyState
	keyHashes    [][sha256.Size]byte
	keyValid     []bool
	stateMu      sync.RWMutex
	inputMu      sync.Mutex
	pageTimer    *time.Timer
	pageTimerMu  sync.Mutex
	refreshMu    sync.Mutex
}

type keyState struct {
	pressed bool
	held    bool
	press   Action
	hold    Action
	downAt  time.Time
	timer   *time.Timer
}

func New(deckDevice *deck.Device, config Config) *App {
	media := NewMediaPlayer(config.MediaPlayer)
	layout := render.DefaultLayout()
	if deckDevice != nil {
		layout = deckDevice.Model().Layout()
	} else if model, ok := deck.ModelByID(config.DeviceModel); ok {
		layout = model.Layout()
	}
	app := &App{
		deck:         deckDevice,
		renderer:     render.NewWithLayout(config.IconDir, layout, config.FontPath),
		media:        media,
		mediaPlayers: map[string]*MediaPlayer{media.Player(): media},
		weather:      NewWeather(config.Location),
		config:       config,
		layout:       layout,
		currentPage:  config.StartPage,
		brightness:   config.Brightness,
	}
	app.ensureKeyBuffers()
	return app
}

func (a *App) Run(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer a.stopTimers()

	if err := a.deck.SetBrightness(a.currentBrightness()); err != nil {
		return err
	}
	if err := a.deck.BeginImageUpload(); err != nil {
		return err
	}
	if err := a.startIPC(runCtx); err != nil {
		return err
	}
	a.setMediaState(a.media.InitialState(runCtx))
	if err := a.Refresh(runCtx); err != nil {
		return err
	}
	a.resetPageTimeout(runCtx)
	mediaUpdates := make(chan MediaState, 4)
	go a.media.Watch(runCtx, mediaUpdates)
	deckErrors := make(chan error, 1)
	go func() {
		err := a.deck.WatchKeys(runCtx, func(key int, pressed bool) {
			a.HandleKeyEvent(runCtx, key, pressed)
		})
		if err != nil && runCtx.Err() == nil {
			deckErrors <- err
		}
	}()

	clockTicker := time.NewTicker(time.Minute)
	defer clockTicker.Stop()
	weatherTicker := time.NewTicker(a.config.WeatherRefresh)
	defer weatherTicker.Stop()
	animationTicker := time.NewTicker(100 * time.Millisecond)
	defer animationTicker.Stop()
	for {
		select {
		case <-runCtx.Done():
			return runCtx.Err()
		case err := <-deckErrors:
			return err
		case state := <-mediaUpdates:
			a.setMediaState(state)
			if err := a.Refresh(runCtx); err != nil {
				return err
			}
		case <-clockTicker.C:
			if err := a.Refresh(runCtx); err != nil {
				return err
			}
		case <-weatherTicker.C:
			if err := a.Refresh(runCtx); err != nil {
				return err
			}
		case <-animationTicker.C:
			if a.displayMode() != 2 && pageHasAnimation(a.page()) {
				if err := a.Refresh(runCtx); err != nil {
					return err
				}
			}
		}
	}
}

func (a *App) stopTimers() {
	a.pageTimerMu.Lock()
	if a.pageTimer != nil {
		a.pageTimer.Stop()
		a.pageTimer = nil
	}
	a.pageTimerMu.Unlock()

	a.inputMu.Lock()
	a.ensureKeyBuffersLocked()
	for key := range a.keyStates {
		if a.keyStates[key].timer != nil {
			a.keyStates[key].timer.Stop()
			a.keyStates[key].timer = nil
		}
	}
	a.inputMu.Unlock()
}

func (a *App) HandleKeyEvent(ctx context.Context, key int, pressed bool) {
	if key < 0 || key >= a.keyCount() {
		fmt.Println("Invalid button:", key)
		return
	}
	a.ensureKeyBuffers()
	if pressed {
		a.handleKeyDown(ctx, key)
		return
	}
	a.handleKeyUp(ctx, key)
}

func (a *App) handleKeyDown(ctx context.Context, key int) {
	fmt.Printf("Button %d pressed\n", key)
	page := a.page()
	if key < 0 || key >= len(page.Buttons) {
		fmt.Println("No action assigned for this key.")
		return
	}

	button := page.Buttons[key]
	if !hasAction(button.Hold) {
		a.handleActionAndRefresh(ctx, button.Press)
		return
	}

	a.inputMu.Lock()
	if state := &a.keyStates[key]; state.timer != nil {
		state.timer.Stop()
	}
	a.keyStates[key] = keyState{
		pressed: true,
		press:   button.Press,
		hold:    button.Hold,
		downAt:  time.Now(),
	}
	holdDuration := time.Duration(a.config.HoldMS) * time.Millisecond
	a.keyStates[key].timer = time.AfterFunc(holdDuration, func() {
		a.fireHold(ctx, key)
	})
	a.inputMu.Unlock()
}

func (a *App) handleKeyUp(ctx context.Context, key int) {
	fmt.Printf("Button %d released\n", key)

	a.inputMu.Lock()
	state := a.keyStates[key]
	a.keyStates[key] = keyState{}
	if state.timer != nil {
		state.timer.Stop()
	}
	a.inputMu.Unlock()

	if !state.pressed || state.held {
		return
	}
	if time.Since(state.downAt) >= time.Duration(a.config.HoldMS)*time.Millisecond {
		a.handleActionAndRefresh(ctx, state.hold)
		return
	}
	a.handleActionAndRefresh(ctx, state.press)
}

func (a *App) fireHold(ctx context.Context, key int) {
	a.inputMu.Lock()
	state := &a.keyStates[key]
	if !state.pressed || state.held {
		a.inputMu.Unlock()
		return
	}
	state.held = true
	hold := state.hold
	a.inputMu.Unlock()

	fmt.Printf("Button %d held\n", key)
	a.handleActionAndRefresh(ctx, hold)
}

func (a *App) handleActionAndRefresh(ctx context.Context, action Action) {
	a.handleAction(ctx, action)
	a.resetPageTimeout(ctx)
	if a.deck == nil {
		return
	}
	if err := a.Refresh(ctx); err != nil {
		fmt.Println("Refresh error:", err)
	}
}

func hasAction(action Action) bool {
	return action.Type != "" || action.Command != "" || action.Page != ""
}

func (a *App) handleAction(ctx context.Context, action Action) {
	switch action.Type {
	case "", "empty":
		return
	case "media":
		a.mediaFor(action.Player).Control(ctx, action.Command)
	case "command":
		runShellCommand(ctx, action.Command)
	case "display_mode":
		if action.Command == "" || action.Command == "cycle" {
			a.cycleDisplayMode()
		}
	case "brightness":
		a.handleBrightnessAction(action)
	case "page":
		pageID := action.Page
		if pageID == "" {
			pageID = action.Command
		}
		if !a.pageExists(pageID) {
			fmt.Println("Unknown page:", pageID)
			return
		}
		a.setCurrentPage(pageID)
	default:
		fmt.Println("Unknown action type:", action.Type)
	}
}

func (a *App) setCurrentPage(pageID string) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.currentPage = pageID
}

func (a *App) pageExists(pageID string) bool {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	_, ok := a.config.Pages[pageID]
	return ok
}

func (a *App) resetPageTimeout(ctx context.Context) {
	pageID, page := a.currentPageSnapshot()

	a.pageTimerMu.Lock()
	if a.pageTimer != nil {
		a.pageTimer.Stop()
		a.pageTimer = nil
	}

	if pageID == "" || pageID == a.config.StartPage {
		a.pageTimerMu.Unlock()
		return
	}
	if page.TimeoutSeconds <= 0 {
		a.pageTimerMu.Unlock()
		return
	}

	timeout := time.Duration(page.TimeoutSeconds) * time.Second
	a.pageTimer = time.AfterFunc(timeout, func() {
		a.returnToStartPage(ctx, pageID)
	})
	a.pageTimerMu.Unlock()
}

func (a *App) returnToStartPage(ctx context.Context, expectedPage string) {
	a.pageTimerMu.Lock()
	if !a.setCurrentPageIf(expectedPage, a.config.StartPage) {
		a.pageTimerMu.Unlock()
		return
	}
	a.pageTimer = nil
	a.pageTimerMu.Unlock()

	if a.deck == nil {
		return
	}
	if err := a.Refresh(ctx); err != nil {
		fmt.Println("Refresh error:", err)
	}
}

func (a *App) cycleDisplayMode() {
	a.stateMu.Lock()
	a.mode = (a.mode + 1) % 4
	mode := a.mode
	brightness := a.brightness
	if mode == 2 {
		brightness = 0
	}
	a.stateMu.Unlock()

	if a.deck == nil {
		return
	}
	if err := a.deck.SetBrightness(brightness); err != nil {
		fmt.Println("Error setting brightness:", err)
	}
}

func (a *App) handleBrightnessAction(action Action) {
	step := action.Step
	if step <= 0 {
		step = 10
	}

	a.stateMu.Lock()
	next := int(a.brightness)
	switch action.Command {
	case "up", "increase":
		next += step
	case "down", "decrease":
		next -= step
	case "set":
		if action.Value == nil {
			fmt.Println("Brightness action set requires value")
			a.stateMu.Unlock()
			return
		}
		next = *action.Value
	default:
		fmt.Println("Unknown brightness command:", action.Command)
		a.stateMu.Unlock()
		return
	}
	next = clampInt(next, 0, 100)
	a.brightness = byte(next)
	mode := a.mode
	a.stateMu.Unlock()

	if mode == 2 || a.deck == nil {
		return
	}
	if err := a.deck.SetBrightness(byte(next)); err != nil {
		fmt.Println("Error setting brightness:", err)
	}
}

func (a *App) Refresh(ctx context.Context) error {
	a.refreshMu.Lock()
	defer a.refreshMu.Unlock()
	a.ensureKeyBuffers()

	page := a.page()
	backgroundPlayer := page.Background.Player
	backgroundState := a.mediaStateFor(ctx, backgroundPlayer)
	album := a.mediaFor(backgroundPlayer).AlbumArt(ctx, backgroundState.ArtURL)
	weather := a.weather.Current(ctx)
	renderPage := a.renderPage(ctx, page)
	mode := a.displayMode()
	mediaStatus := a.primaryMediaStatus()
	keys, err := a.renderer.ComposePage(renderPage, album, mode, mediaStatus, weather, time.Now())
	if err != nil {
		return err
	}
	needsUpload := false
	bmps := make([][]byte, len(keys))
	hashes := make([][sha256.Size]byte, len(keys))
	for key, img := range keys {
		payload, err := a.keyPayload(img)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(payload)
		bmps[key] = payload
		hashes[key] = hash
		if a.keyValid[key] && a.keyHashes[key] == hash {
			continue
		}
		needsUpload = true
	}
	if !needsUpload {
		return nil
	}
	for key, bmp := range bmps {
		if a.keyValid[key] && a.keyHashes[key] == hashes[key] {
			continue
		}
		if err := a.deck.UploadKeyImage(key, bmp); err != nil {
			return err
		}
		a.keyHashes[key] = hashes[key]
		a.keyValid[key] = true
	}
	return nil
}

func (a *App) keyPayload(img image.Image) ([]byte, error) {
	if a.deck != nil && a.deck.Model().ImageFormat == deck.ImageFormatJPEG {
		return a.renderer.KeyToJPEG(img)
	}
	return a.renderer.KeyToBMP(img), nil
}

func (a *App) page() Page {
	a.stateMu.RLock()
	currentPage := a.currentPage
	page, ok := a.config.Pages[currentPage]
	if ok {
		a.stateMu.RUnlock()
		return page
	}

	startPage := a.config.StartPage
	page, ok = a.config.Pages[startPage]
	a.stateMu.RUnlock()
	if ok {
		a.setCurrentPage(startPage)
		return page
	}
	return Page{}
}

func (a *App) currentPageSnapshot() (string, Page) {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	pageID := a.currentPage
	return pageID, a.config.Pages[pageID]
}

func (a *App) setCurrentPageIf(expectedPage, nextPage string) bool {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	if a.currentPage != expectedPage {
		return false
	}
	a.currentPage = nextPage
	return true
}

func (a *App) displayMode() int {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.mode
}

func (a *App) currentBrightness() byte {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.brightness
}

func (a *App) setMediaState(state MediaState) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.mediaState = state
}

func (a *App) primaryMediaStatus() string {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.mediaState.Status
}

func (a *App) renderPage(ctx context.Context, page Page) render.Page {
	renderPage := render.Page{
		Background: page.Background,
		Buttons:    make([]render.Button, len(page.Buttons)),
	}
	for i, button := range page.Buttons {
		layers := make([]render.Layer, len(button.Layers))
		copy(layers, button.Layers)
		for j := range layers {
			if layers[j].Type == "media_play_pause" {
				layers[j].Playback = a.mediaStateFor(ctx, layers[j].Player).Status
			}
		}
		renderPage.Buttons[i] = render.Button{
			Layers: layers,
		}
	}
	return renderPage
}

func (a *App) mediaStateFor(ctx context.Context, player string) MediaState {
	media := a.mediaFor(player)
	if media.Player() == a.media.Player() {
		a.stateMu.RLock()
		defer a.stateMu.RUnlock()
		return a.mediaState
	}
	return media.InitialState(ctx)
}

func (a *App) mediaFor(player string) *MediaPlayer {
	key := canonicalPlayer(player, a.config.MediaPlayer)
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	if media, ok := a.mediaPlayers[key]; ok {
		return media
	}
	media := NewMediaPlayer(key)
	a.mediaPlayers[media.Player()] = media
	return media
}

func pageHasAnimation(page Page) bool {
	if page.Background.Effect.Type != "" {
		return true
	}
	for _, button := range page.Buttons {
		for _, layer := range button.Layers {
			if layer.Type == "animation" || layer.Effect.Type != "" {
				return true
			}
		}
	}
	return false
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func (a *App) keyCount() int {
	if count := a.layout.KeyCount(); count > 0 {
		return count
	}
	return render.DefaultLayout().KeyCount()
}

func (a *App) ensureKeyBuffers() {
	a.inputMu.Lock()
	defer a.inputMu.Unlock()
	a.ensureKeyBuffersLocked()
}

func (a *App) ensureKeyBuffersLocked() {
	count := a.keyCount()
	if len(a.keyStates) != count {
		next := make([]keyState, count)
		copy(next, a.keyStates)
		a.keyStates = next
	}
	if len(a.keyHashes) != count {
		next := make([][sha256.Size]byte, count)
		copy(next, a.keyHashes)
		a.keyHashes = next
	}
	if len(a.keyValid) != count {
		next := make([]bool, count)
		copy(next, a.keyValid)
		a.keyValid = next
	}
}
