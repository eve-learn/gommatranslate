package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/eve-learn/gommatranslate"
)

func main() {
	ctx := context.Background()
	client, err := gommatranslate.New(gommatranslate.Options{
		ModelID: "q4_k_m",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	if err := client.Download(ctx, func(p gommatranslate.DownloadProgress) {
		fmt.Fprintf(os.Stderr, "\r%5.1f%%  %s", p.Percent, p.Message)
	}); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintln(os.Stderr)

	if err := client.Start(ctx); err != nil {
		log.Fatal(err)
	}

	text := "Hello world"
	if len(os.Args) > 1 {
		text = os.Args[1]
	}
	out, err := client.Translate(ctx, text, "en", "es")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(out)
}
