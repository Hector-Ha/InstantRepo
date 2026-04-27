package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"instantrepo/internal/api"
	"instantrepo/internal/domain"
	"instantrepo/internal/service"
)

func main() {
	var (
		serveAddr = flag.String("serve", "", "HTTP listen address, for example :8080")
		repoURL   = flag.String("repo", "", "GitHub repository URL to analyze")
		localPath = flag.String("path", "", "Local repository path to analyze")
		stepID    = flag.String("step", "", "Plan step ID to execute after analysis")
		approve   = flag.Bool("approve", false, "Allow execution of risky steps that require approval")
	)
	flag.Parse()

	app := service.NewAppService()

	if *serveAddr != "" {
		server := &http.Server{
			Addr:              *serveAddr,
			Handler:           api.NewServer(app),
			ReadHeaderTimeout: 10 * time.Second,
		}

		log.Printf("instantrepo listening on %s", *serveAddr)
		log.Fatal(server.ListenAndServe())
	}

	if *repoURL == "" && *localPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	if *stepID != "" {
		resp, err := app.Execute(context.Background(), domain.ExecuteRequest{
			RepoURL:      *repoURL,
			LocalPath:    *localPath,
			StepID:       *stepID,
			ApproveRisky: *approve,
		})
		if err != nil {
			log.Fatal(err)
		}
		if err := enc.Encode(resp); err != nil {
			log.Fatal(fmt.Errorf("encode response: %w", err))
		}
		return
	}

	resp, err := app.Analyze(context.Background(), domain.AnalyzeRequest{
		RepoURL:   *repoURL,
		LocalPath: *localPath,
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := enc.Encode(resp); err != nil {
		log.Fatal(fmt.Errorf("encode response: %w", err))
	}
}
