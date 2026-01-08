package main

import (
	"fmt"
	"log"
)

func main() {
	log.SetPrefix("grawl: ")
	log.SetFlags(0)

	res, err := fetch("https://books.toscrape.com/")

	if err != nil {
		log.Fatal(err)
	}

	links := extractLink(res)

	for _, val := range links {
		fmt.Println(val)
	}
}
