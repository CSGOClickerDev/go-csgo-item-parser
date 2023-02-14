package csgo

import (
	"fmt"
	"strings"
)

// getIconSet will process all icons included within the items data and
// attempt to extract the item and paintkit IDs for the target item IDs
// included in itemIds.
//
// e.g. look for all paintkits that are available for the item
// weapon_knife_css.
func getIconSet[T any](items map[string]interface{}, itemIds map[string]T) (*itemPaintkitSet, error) {

	response := &itemPaintkitSet{}

	icons, err := crawlToType[map[string]interface{}](items, "alternate_icons2", "weapon_icons")
	if err != nil {
		return nil, fmt.Errorf("unable to locate weapon_icons: %s", err.Error())
	}

	for index, data := range icons {

		iconMap, ok := data.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected alternate_icons2 format %s", index)
		}

		iconPath, err := crawlToType[string](iconMap, "icon_path")
		if err != nil {
			return nil, err
		}

		// each weapon skin appears in icons 3 times, by including only the "..._light"
		// ones we are eliminating duplicates.
		if !strings.HasSuffix(iconPath, "_light") {
			continue
		}

		targetId := findLongestIdMatch(itemIds, iconPath)
		if targetId == "" {
			continue
		}

		itemId, paintkitId, err := getItemPaintkitFromIconPath(targetId, iconPath)
		if err != nil {
			return nil, err
		}

		// if entry doesn't already exist, create map to prevent nil dereference
		response.add(itemId, paintkitId)
	}

	return response, nil
}

// findLongestIdMatch will take a provided icon path and locate the
// longest matching id from ids within the path.
//
// e.g. with the ids: { test_weapon_knife, test_weapon_knife_karambit }
// and the path "icon/path/test_weapon_knife_karambit",
// test_weapon_knife_karambit will be returned.
func findLongestIdMatch[T any](ids map[string]T, path string) string {

	longest := ""

	for id, _ := range ids {

		if !strings.Contains(path, id) {
			continue
		}

		if len(id) > len(longest) {
			longest = id
		}
	}

	return longest
}

// getItemPaintkitFromIconPath will extract from the provided path, the
// paintkit ID. itemID is required to distinguish the paintkit from the
// weapon.
func getItemPaintkitFromIconPath(itemId string, path string) (string, string, error) {

	pathTail := strings.TrimPrefix(path, "econ/default_generated/")
	components := strings.Split(pathTail, "_")

	for i := 0; i < len(components); i++ {

		iId := strings.Join(components[:i], "_")
		pkId := strings.Join(components[i:len(components)-1], "_") // drop last component (as light, medium or heavy)

		if iId != itemId {
			continue
		}

		return iId, pkId, nil
	}

	return "", "", fmt.Errorf("unable to derive weapon and paintkit from icon path: %s", path)
}
