package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

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

	// Batch a couple of translation calls and print the time each call takes
	type translationJob struct {
		text       string
		sourceLang string
		targetLang string
	}

	jobs := []translationJob{
		{"Hello world", "en", "es"},
		{"How are you?", "en", "fr"},
	}
	// If the user passed args, use those as the texts for the batch
	if len(os.Args) > 1 {
		jobs = nil
		for _, arg := range os.Args[1:] {
			jobs = append(jobs, translationJob{arg, "en", "es"})
		}
	}

	for idx, job := range jobs {
		fmt.Fprintf(os.Stderr, "Translating (%d): %q -> %s...\n", idx+1, job.text, job.targetLang)
		start := time.Now()
		out, err := client.Translate(ctx, job.text, job.sourceLang, job.targetLang)
		elapsed := time.Since(start)
		if err != nil {
			return err
		}
		fmt.Printf("Result (%d): %s\nTranslation took: %v\n\n", idx+1, out, elapsed)
	}

	return nil
}
