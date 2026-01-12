package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
)

func main() {
	log.SetPrefix("grawl: ")
	log.SetFlags(0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	base, err := url.Parse("https://books.toscrape.com/")

	if err != nil {
		log.Fatal(err)
	}

	res, err := fetch(base.String())

	if err != nil {
		log.Fatal(err)
	}

	links := extractLink(ctx, base, res)

	for val := range links {
		fmt.Println(val)

		res, err := fetch(val)

		if err != nil {
			log.Fatal(err)
		}

		link, err := url.Parse(val)

		if err != nil {
			log.Fatal(err)
		}

		for val1 := range extractLink(ctx, link, res) {
			fmt.Println(val1)
		}
	}
}
