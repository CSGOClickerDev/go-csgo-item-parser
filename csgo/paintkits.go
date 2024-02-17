package csgo

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

var (
	defaultMinFloat = decimal.RequireFromString("0.06")
	defaultMaxFloat = decimal.RequireFromString("0.8")
)

// Paintkit represents the image details of a skin, i.e. the available float
// range the skin can be in. Every entities.Skin has an associated Paintkit.
type Paintkit struct {
	Id          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	RarityId    string          `json:"rarityId"`
	MinFloat    decimal.Decimal `json:"minFloat"`
	MaxFloat    decimal.Decimal `json:"maxFloat"`
}

// mapToPaintkit converts the provided map into a Paintkit providing
// all required parameters are present and of the correct type.
func mapToPaintkit(data map[string]interface{}, language *language) (*Paintkit, error) {

	response := &Paintkit{
		RarityId: "common", // common is "default" rarity
		MinFloat: defaultMinFloat,
		MaxFloat: defaultMaxFloat,
	}

	// get Name
	if val, err := crawlToType[string](data, "name"); err != nil {
		return nil, errors.New("Id (name) missing from Paintkit")
	} else {
		response.Id = val
	}

	// get language Name Id
	if val, ok := data["description_tag"].(string); ok {
		name, err := language.lookup(val)
		if err != nil {
			return nil, err
		}

		response.Name = name
	}

	// get language Description Id
	if val, ok := data["description_string"].(string); ok {
		description, err := language.lookup(val)
		if err != nil {
			return nil, err
		}

		// Remove HTML tags
		re := regexp.MustCompile(`<[^>]*>`)
		withoutTags := re.ReplaceAllString(description, "")

		// Remove escaped characters and replace with a single space
		re = regexp.MustCompile(`(\\n)+`)
		description = re.ReplaceAllString(withoutTags, " ")

		response.Description = description
	}

	// get min float
	if val, ok := data["wear_remap_min"].(string); ok {
		if valFloat, err := decimal.NewFromString(val); err == nil {
			response.MinFloat = valFloat
		} else {
			return nil, errors.New("Paintkit has non-float min float value (wear_remap_min)")
		}
	}

	// get max float
	if val, ok := data["wear_remap_max"].(string); ok {
		if valFloat, err := decimal.NewFromString(val); err == nil {
			response.MaxFloat = valFloat
		} else {
			return nil, errors.New("Paintkit has non-float max float value (wear_remap_max)")
		}
	}

	return response, nil
}

// getPaintkits gathers all Paintkits in the provided items data and returns them
// as map[paintkitId]Paintkit.
func (c *csgoItems) getPaintkits() (map[string]*Paintkit, error) {

	response := map[string]*Paintkit{}

	// This logic should definitely be refactored, & moved into separate functions. But yolo.

	globalRarities, err := c.getRarities()
	if err != nil {
		panic(err)
	}
	raritiesSet := map[string]bool{}
	for rarity, _ := range globalRarities {
		raritiesSet[rarity] = true
	}

	clientLootList, err := crawlToType[map[string]interface{}](c.items, "client_loot_lists")
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to locate client_loot_lists amongst items: %s", err.Error()))
	}

	// get list of all collection rarities in loot list
	raritiesLootList := []string{}
	for key, _ := range clientLootList {
		substr := strings.Split(key, "_")
		lastItemIndex := len(substr) - 1
		if raritiesSet[substr[lastItemIndex]] {
			raritiesLootList = append(raritiesLootList, key)
		}
	}

	delimiterFunc := func(c rune) bool {
		return c == '[' || c == ']'
	}

	// goal is item_name -> rarity
	itemToRarity := map[string]string{}

	for _, rarityCollection := range raritiesLootList {
		itemPaintKit, err := crawlToType[map[string]interface{}](clientLootList, rarityCollection)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("unable to locate rarityCollection amongst items: %s", err.Error()))
		}

		for key, _ := range itemPaintKit {
			// Split keystring by brackets delimiters:
			substr := strings.FieldsFunc(key, delimiterFunc)
			for i, s := range substr {
				if i == 0 {
					paintKitId := s
					substr := strings.Split(rarityCollection, "_")
					lastItemIndex := len(substr) - 1
					rarity := substr[lastItemIndex]

					itemToRarity[paintKitId] = rarity
				}
			}
		}
	}

	rarities, err := crawlToType[map[string]interface{}](c.items, "paint_kits_rarity")
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to extract paint_kits_rarity: %s", err.Error()))
	}

	// if loot list rarity does not equal paint kit rarity, override it with loot list rarity.
	for key, val := range rarities {
		if val.(string) != itemToRarity[key] {
			rarities[key] = itemToRarity[key]
		}
	}

	kits, err := crawlToType[map[string]interface{}](c.items, "paint_kits")
	if err != nil {
		return nil, errors.Wrap(err, "unable to locate paint_kits in provided items")
	}

	for index, kit := range kits {
		mKit, ok := kit.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected Paintkit layout in paint_kits for index (%s)", index)
		}

		converted, err := mapToPaintkit(mKit, c.language)
		if err != nil {
			return nil, err
		}

		if converted.Id == "workshop_default" {
			continue
		}

		if rarity, ok := rarities[converted.Id].(string); ok {
			converted.RarityId = rarity
		}

		// if default paintkit, manually set rarity
		if converted.Id == "default" {
			converted.RarityId = "default"
		}

		response[converted.Id] = converted
	}

	return response, nil
}
