package main

import (
	"context"
	"fmt"
	"log"
)

func main() {
	log.SetPrefix("grawl: ")
	log.SetFlags(0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rootLink := "https://books.toscrape.com/"

	res, err := fetch(rootLink)

	if err != nil {
		log.Fatal(err)
	}

	links := extractLink(ctx, rootLink, res)

	for val := range links {
		fmt.Println(val)

		res, err := fetch(val)

		if err != nil {
			log.Fatal(err)
		}

		for val1 := range extractLink(ctx, val, res) {
			fmt.Println(val1)
		}
	}

	// TODO: Make it able to keep extracting links until the channel is empty
}
