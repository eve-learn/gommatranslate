package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/eve-learn/gommatranslate"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
	client, err := gommatranslate.New(gommatranslate.Options{
		ModelID: "q4_k_m",
	})
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.Download(ctx, func(p gommatranslate.DownloadProgress) {
		fmt.Fprintf(os.Stderr, "\r%5.1f%%  %s\033[K", p.Percent, p.Message)
	}); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr)

	fmt.Fprintln(os.Stderr, "Starting runtime...")
	if err := client.Start(ctx); err != nil {
		return err
	}

	text := "Hello world"
	if len(os.Args) > 1 {
		text = os.Args[1]
	}
	out, err := client.Translate(ctx, text, "en", "es")
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}
