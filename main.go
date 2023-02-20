package main

import (
	"flag"
	"fmt"
	"github.com/rustedturnip/go-csgo-item-parser/csgo"

	"github.com/rustedturnip/go-csgo-item-parser/parser"
)

var (
	csgoItemsLocation   string
	csgoEnglishLocation string
)

func init() {
	flag.StringVar(&csgoItemsLocation, "csgo-items", "/Users/samuel/Downloads/items_game.txt", "the path to the csgo_items.txt file")
	flag.StringVar(&csgoEnglishLocation, "csgo-english", "/Users/samuel/Downloads/csgo_english.txt", "the path to the csgo_english.txt file")
}

func main() {
	flag.Parse()

	result, err := parser.Parse(csgoEnglishLocation)
	if err != nil {
		panic(err)
	}

	resultTwo, err := parser.Parse(csgoItemsLocation)
	if err != nil {
		panic(err)
	}

	allItems, err := csgo.New(result, resultTwo)
	if err != nil {
		panic(err)
	}

	// TODO testing

	for _, paintkit := range allItems.Paintkits {
		fmt.Println(paintkit.Name)
	}

	// TODO !testing
}
