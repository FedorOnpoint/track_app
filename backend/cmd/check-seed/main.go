package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/FedorOnpoint/track_app/backend/internal/seedcontent"
)

var errUnavailable = errors.New("some links could not be verified; retry when the network or remote service is available")
var errBroken = errors.New("invalid or broken links found")

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, errUnavailable) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("check-seed", flag.ContinueOnError)
	catalog := flags.Bool("catalog", false, "check a complete production catalog (13 categories, 80–120 topics and cross-category relationships)")
	checkLinks := flags.Bool("links", false, "check external HTTPS links (unavailable services exit with code 2)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() == 0 {
		return errors.New("usage: check-seed [-links] category.json [category.json ...]")
	}
	var documents []seedcontent.Document
	urls := map[string]bool{}
	encoder := json.NewEncoder(output)
	for _, path := range flags.Args() {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		document, err := seedcontent.Decode(file)
		file.Close()
		if err == nil {
			err = seedcontent.Validate(document)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := encoder.Encode(struct {
			File   string `json:"file"`
			Topics int    `json:"topics"`
			Valid  bool   `json:"valid"`
		}{path, len(document.Topics), true}); err != nil {
			return err
		}
		documents = append(documents, document)
		for _, topic := range document.Topics {
			for _, resource := range topic.Resources {
				urls[resource.URL] = true
			}
			for _, practice := range topic.Practice {
				if practice.URL != "" {
					urls[practice.URL] = true
				}
			}
		}
	}
	if *catalog {
		if err := seedcontent.ValidateCatalog(documents, true); err != nil {
			return err
		}
	}
	if !*checkLinks {
		return nil
	}
	addresses := make([]string, 0, len(urls))
	for address := range urls {
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)
	checker := seedcontent.NewLinkChecker(nil)
	broken, unavailable := false, false
	for _, address := range addresses {
		if err := ctx.Err(); err != nil {
			return err
		}
		result := checker.Check(ctx, address)
		if err := encoder.Encode(result); err != nil {
			return err
		}
		broken = broken || result.Status == seedcontent.LinkBroken || result.Status == seedcontent.LinkInvalid
		unavailable = unavailable || result.Status == seedcontent.LinkUnavailable
	}
	if broken {
		return errBroken
	}
	if unavailable {
		return errUnavailable
	}
	return nil
}
