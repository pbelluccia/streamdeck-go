package deck

import (
	"fmt"
	"sort"
	"strings"

	"streamdeck-go/internal/render"
)

const (
	VendorID    = "00000FD9"
	MaxKeyCount = 32
)

type Protocol string

const (
	ProtocolMini    Protocol = "mini"
	ProtocolGeneral Protocol = "general"
)

type ImageFormat string

const (
	ImageFormatBMP  ImageFormat = "bmp"
	ImageFormatJPEG ImageFormat = "jpeg"
)

type Model struct {
	ID          string
	Name        string
	ProductID   string
	Columns     int
	Rows        int
	KeyWidth    int
	KeyHeight   int
	Protocol    Protocol
	ImageFormat ImageFormat
}

func (m Model) KeyCount() int {
	return m.Columns * m.Rows
}

func (m Model) Layout() render.Layout {
	return render.Layout{
		Columns:   m.Columns,
		Rows:      m.Rows,
		KeyWidth:  m.KeyWidth,
		KeyHeight: m.KeyHeight,
	}
}

func (m Model) String() string {
	if m.Name != "" {
		return m.Name
	}
	return m.ID
}

func ModelByID(id string) (Model, bool) {
	id = normalizeID(id)
	if id == "" || id == "auto" {
		return defaultModels["mini"], true
	}
	model, ok := defaultModels[id]
	return model, ok
}

func ModelForProductID(productID string) (Model, bool) {
	productID = strings.ToUpper(strings.TrimSpace(productID))
	productID = strings.TrimPrefix(productID, "0X")
	for len(productID) < 8 {
		productID = "0" + productID
	}
	model, ok := modelsByProductID[productID]
	return model, ok
}

func ValidateModelID(id string) error {
	id = normalizeID(id)
	if id == "" || id == "auto" {
		return nil
	}
	if _, ok := defaultModels[id]; !ok {
		return fmt.Errorf("settings.model must be auto, mini, classic, or xl")
	}
	return nil
}

func NormalizeModelID(id string) string {
	id = normalizeID(id)
	if id == "" {
		return "auto"
	}
	return id
}

func SupportedModels() []Model {
	models := make([]Model, 0, len(modelsByProductID))
	for _, model := range modelsByProductID {
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].ID == models[j].ID {
			return models[i].ProductID < models[j].ProductID
		}
		return models[i].ID < models[j].ID
	})
	return models
}

func normalizeID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	id = strings.ReplaceAll(id, "_", "-")
	switch id {
	case "", "auto":
		return id
	case "stream-deck-mini", "mini", "6", "6-key":
		return "mini"
	case "stream-deck-classic", "classic", "15", "15-key", "mk2", "mk.2":
		return "classic"
	case "stream-deck-xl", "xl", "32", "32-key":
		return "xl"
	default:
		return id
	}
}

var defaultModels = map[string]Model{
	"mini": {
		ID:          "mini",
		Name:        "Stream Deck Mini",
		ProductID:   "00000063",
		Columns:     3,
		Rows:        2,
		KeyWidth:    80,
		KeyHeight:   80,
		Protocol:    ProtocolMini,
		ImageFormat: ImageFormatBMP,
	},
	"classic": {
		ID:          "classic",
		Name:        "Stream Deck Classic",
		ProductID:   "0000006D",
		Columns:     5,
		Rows:        3,
		KeyWidth:    72,
		KeyHeight:   72,
		Protocol:    ProtocolGeneral,
		ImageFormat: ImageFormatJPEG,
	},
	"xl": {
		ID:          "xl",
		Name:        "Stream Deck XL",
		ProductID:   "0000006C",
		Columns:     8,
		Rows:        4,
		KeyWidth:    96,
		KeyHeight:   96,
		Protocol:    ProtocolGeneral,
		ImageFormat: ImageFormatJPEG,
	},
}

var modelsByProductID = map[string]Model{
	"00000063": withProduct(defaultModels["mini"], "Stream Deck Mini", "00000063"),
	"00000090": withProduct(defaultModels["mini"], "Stream Deck Mini 2022", "00000090"),
	"000000B3": withProduct(defaultModels["mini"], "Stream Deck Mini Discord", "000000B3"),
	"000000B8": withProduct(defaultModels["mini"], "Stream Deck 6-Key Module", "000000B8"),

	"0000006D": withProduct(defaultModels["classic"], "Stream Deck 2019", "0000006D"),
	"00000080": withProduct(defaultModels["classic"], "Stream Deck Mk.2", "00000080"),
	"000000A5": withProduct(defaultModels["classic"], "Stream Deck Mk.2 Scissor Keys", "000000A5"),
	"000000B9": withProduct(defaultModels["classic"], "Stream Deck 15-Key Module", "000000B9"),

	"0000006C": withProduct(defaultModels["xl"], "Stream Deck XL", "0000006C"),
	"0000008F": withProduct(defaultModels["xl"], "Stream Deck XL 2022", "0000008F"),
	"000000BA": withProduct(defaultModels["xl"], "Stream Deck Module 32", "000000BA"),
}

func withProduct(model Model, name string, productID string) Model {
	model.Name = name
	model.ProductID = productID
	return model
}
