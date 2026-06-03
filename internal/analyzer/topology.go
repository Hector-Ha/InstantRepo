package analyzer

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"instantrepo/internal/domain"
	"instantrepo/internal/util"
)

func detectAppTopology(repoPath string, env domain.EnvironmentConfig, services []domain.ServiceDependency) domain.AppTopology {
	signals := []domain.AppTopologySignal{}
	for _, manifest := range findPackageManifests(repoPath) {
		signals = append(signals, nodeTopologySignals(repoPath, manifest)...)
	}
	for _, envVar := range env.Variables {
		if isPortEnvVar(envVar.Name) {
			if port, err := strconv.Atoi(strings.TrimSpace(envVar.SuggestedValue)); err == nil && port > 0 {
				signals = appendTopologySignal(signals, domain.AppTopologySignal{
					Kind:       "port",
					TargetDir:  envVar.TargetDir,
					Port:       port,
					Confidence: 0.82,
					Evidence:   envVar.Name + " env requirement",
				})
			}
		}
		if isFrontendEnvVar(envVar.Name) {
			signals = appendTopologySignal(signals, domain.AppTopologySignal{
				Kind:       "frontend",
				TargetDir:  envVar.TargetDir,
				Confidence: 0.72,
				Evidence:   envVar.Name + " env usage",
			})
		}
		if envVar.Service != "" {
			kind := "external-provider"
			if isLocalDataService(envVar.Service) {
				kind = dataServiceKind(envVar.Service)
			}
			signals = appendTopologySignal(signals, domain.AppTopologySignal{
				Kind:       kind,
				TargetDir:  envVar.TargetDir,
				Service:    envVar.Service,
				Provider:   envVar.Service,
				Confidence: 0.7,
				Evidence:   envVar.Name + " env requirement",
			})
		}
	}
	for _, service := range services {
		if !isLocalDataService(service.Name) {
			continue
		}
		if service.Scope != "local" && service.Provisioning != "docker-compose" {
			continue
		}
		signals = appendTopologySignal(signals, domain.AppTopologySignal{
			Kind:       dataServiceKind(service.Name),
			Service:    service.Name,
			Confidence: 0.9,
			Evidence:   service.Source,
		})
	}
	if hasTopologyKind(signals, "frontend") && hasTopologyKind(signals, "backend") {
		signals = appendTopologySignal(signals, domain.AppTopologySignal{
			Kind:       "fullstack",
			Confidence: 0.86,
			Evidence:   "frontend and backend signals found",
		})
	}
	return domain.AppTopology{Signals: signals}
}

func isPortEnvVar(name string) bool {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "PORT", "APP_PORT", "API_PORT", "BACKEND_PORT", "SERVER_PORT":
		return true
	default:
		return false
	}
}

func findPackageManifests(repoPath string) []string {
	manifests := []string{}
	_ = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", ".git", "vendor", "build", "dist", ".next":
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "package.json" {
			manifests = append(manifests, path)
		}
		return nil
	})
	return manifests
}

func nodeTopologySignals(repoPath, manifestPath string) []domain.AppTopologySignal {
	var pkg struct {
		Scripts         map[string]string `json:"scripts"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal([]byte(util.ReadTextFile(manifestPath)), &pkg); err != nil {
		return nil
	}
	targetDir := filepath.Dir(manifestPath)
	rel := relativeEnvSourcePath(repoPath, targetDir)
	deps := map[string]bool{}
	for name := range pkg.Dependencies {
		deps[strings.ToLower(name)] = true
	}
	for name := range pkg.DevDependencies {
		deps[strings.ToLower(name)] = true
	}
	signals := []domain.AppTopologySignal{}
	hasNext := deps["next"]
	if hasAnyDep(deps, "vite", "react", "next", "@vitejs/plugin-react", "vue", "svelte") || folderSuggests(rel, "web", "client", "frontend") {
		signals = append(signals, domain.AppTopologySignal{
			Kind:       "frontend",
			TargetDir:  targetDir,
			Confidence: topologyConfidence(hasAnyDep(deps, "vite", "react", "next", "vue", "svelte")),
			Evidence:   relativeEnvSourcePath(repoPath, manifestPath),
		})
	}
	if hasNext {
		signals = append(signals, domain.AppTopologySignal{
			Kind:       "fullstack",
			TargetDir:  targetDir,
			Confidence: 0.9,
			Evidence:   relativeEnvSourcePath(repoPath, manifestPath),
		})
	}
	if hasAnyDep(deps, "express", "fastify", "koa", "@nestjs/core", "hono") || folderSuggests(rel, "api", "server", "backend") {
		signals = append(signals, domain.AppTopologySignal{
			Kind:       "backend",
			TargetDir:  targetDir,
			Confidence: topologyConfidence(hasAnyDep(deps, "express", "fastify", "koa", "@nestjs/core", "hono")),
			Evidence:   relativeEnvSourcePath(repoPath, manifestPath),
		})
	}
	for _, script := range pkg.Scripts {
		lower := strings.ToLower(script)
		if strings.Contains(lower, "worker") || strings.Contains(lower, "queue") {
			signals = append(signals, domain.AppTopologySignal{
				Kind:       "worker",
				TargetDir:  targetDir,
				Confidence: 0.72,
				Evidence:   "package.json worker script",
			})
			break
		}
	}
	return signals
}

func appendTopologySignal(signals []domain.AppTopologySignal, signal domain.AppTopologySignal) []domain.AppTopologySignal {
	if strings.TrimSpace(signal.Kind) == "" {
		return signals
	}
	for i, existing := range signals {
		if existing.Kind == signal.Kind && existing.TargetDir == signal.TargetDir && existing.Service == signal.Service && existing.Provider == signal.Provider {
			if signal.Confidence > existing.Confidence {
				signals[i] = signal
			}
			return signals
		}
	}
	return append(signals, signal)
}

func hasTopologyKind(signals []domain.AppTopologySignal, kind string) bool {
	return slices.ContainsFunc(signals, func(signal domain.AppTopologySignal) bool {
		return signal.Kind == kind
	})
}

func hasAnyDep(deps map[string]bool, names ...string) bool {
	for _, name := range names {
		if deps[name] {
			return true
		}
	}
	return false
}

func folderSuggests(path string, names ...string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	for _, name := range names {
		if lower == name || strings.Contains(lower, "/"+name) || strings.Contains(lower, name+"/") {
			return true
		}
	}
	return false
}

func topologyConfidence(hasDependency bool) float64 {
	if hasDependency {
		return 0.9
	}
	return 0.45
}

func isFrontendEnvVar(name string) bool {
	upper := strings.ToUpper(name)
	return strings.HasPrefix(upper, "VITE_") || strings.HasPrefix(upper, "NEXT_PUBLIC_") || strings.HasPrefix(upper, "REACT_APP_")
}

func isLocalDataService(service string) bool {
	switch service {
	case "postgres", "mongodb", "redis", "mysql":
		return true
	default:
		return false
	}
}

func dataServiceKind(service string) string {
	if service == "redis" {
		return "cache"
	}
	return "database"
}
