package envcatalog

import (
	"embed"
	"encoding/json"
)

//go:embed repo_edge_cases.json
var repoEdgeCaseFiles embed.FS

type RepoEdgeCaseCatalog struct {
	SchemaVersion string             `json:"schemaVersion"`
	Rules         []RepoEdgeCaseRule `json:"rules"`
}

type RepoEdgeCaseRule struct {
	ID       string                `json:"id"`
	Repo     RepoEdgeCaseMatcher   `json:"repo"`
	Manifest RepoEdgeCaseManifest  `json:"manifest"`
	Env      []RepoEdgeCaseDefault `json:"env"`
}

type RepoEdgeCaseMatcher struct {
	Remotes      []string `json:"remotes"`
	ProjectNames []string `json:"projectNames"`
}

type RepoEdgeCaseManifest struct {
	Path              string `json:"path"`
	VersionConstraint string `json:"versionConstraint"`
}

type RepoEdgeCaseDefault struct {
	TargetDir    string   `json:"targetDir"`
	Name         string   `json:"name"`
	DefaultValue string   `json:"defaultValue"`
	ValueClass   string   `json:"valueClass"`
	Confidence   float64  `json:"confidence"`
	Instructions []string `json:"instructions"`
}

func DefaultRepoEdgeCases() RepoEdgeCaseCatalog {
	raw, err := repoEdgeCaseFiles.ReadFile("repo_edge_cases.json")
	if err != nil {
		return RepoEdgeCaseCatalog{}
	}
	var catalog RepoEdgeCaseCatalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		return RepoEdgeCaseCatalog{}
	}
	return catalog
}
