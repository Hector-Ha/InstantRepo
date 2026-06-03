package service

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"instantrepo/internal/domain"
)

func (m *EnvDraftManager) applyDevDefaultAllocation(repoPath, targetPath string, item domain.EnvVarRequirement, value *domain.EnvDraftValue) {
	if value == nil {
		return
	}
	name := strings.ToUpper(strings.TrimSpace(item.Name))
	targetDir := item.TargetDir
	if targetDir == "" {
		targetDir = filepath.Dir(targetPath)
	}
	if value.ValueClass == domain.EnvValueClassServiceCredential || value.ValueClass == domain.EnvValueClassProviderConfig {
		return
	}
	if item.DefaultSource != "" && value.Provenance.Source == item.DefaultSource && strings.TrimSpace(value.Value) != "" {
		return
	}
	if isBackendPortEnv(name) && hasTopologySignalForDir(item, "backend") {
		port := m.assignedPortFromEvidence(repoPath, targetDir, "backend", 8080, item.SuggestedValue, value)
		value.Value = strconv.Itoa(port)
		value.ValueClass = domain.EnvValueClassDevDefault
		value.Confidence = maxConfidence(value.Confidence, topologyConfidenceForItem(item, "backend"))
		value.Provenance.Source = domain.EnvValueSourceAllocator
		return
	}
	if isBackendPortEnv(name) && hasTopologySignalForDir(item, "fullstack") {
		port := m.assignedPortFromEvidence(repoPath, targetDir, "app", 3000, item.SuggestedValue, value)
		value.Value = strconv.Itoa(port)
		value.ValueClass = domain.EnvValueClassDevDefault
		value.Confidence = maxConfidence(value.Confidence, topologyConfidenceForItem(item, "fullstack"))
		value.Provenance.Source = domain.EnvValueSourceAllocator
		return
	}
	if isAppURLEnv(name) && hasTopologySignalForDir(item, "fullstack") {
		port := m.assignedPortFromTargetEvidence(repoPath, targetDir, "app", 3000, item.TopologySignals, value)
		value.Value = "http://localhost:" + strconv.Itoa(port)
		value.ValueClass = domain.EnvValueClassDevDefault
		value.Confidence = maxConfidence(value.Confidence, topologyConfidenceForItem(item, "fullstack"))
		value.Provenance.Source = domain.EnvValueSourceAllocator
		return
	}
	if isFrontendAppURLEnv(name) && hasTopologySignalForDir(item, "backend") {
		if frontendDir, ok := frontendTargetDir(item); ok {
			port := m.assignedPort(repoPath, frontendDir, "frontend", frontendPreferredPort(item, frontendDir), false)
			value.Value = "http://localhost:" + strconv.Itoa(port)
			value.ValueClass = domain.EnvValueClassDevDefault
			value.Confidence = maxConfidence(value.Confidence, 0.82)
			value.Provenance.Source = domain.EnvValueSourceAllocator
			return
		}
	}
	if isFrontendAPIURLEnv(name) && hasTopologySignalForDir(item, "frontend") {
		if backendDir, ok := backendTargetDir(item); ok {
			port := m.assignedPortFromTargetEvidence(repoPath, backendDir, "backend", 8080, item.TopologySignals, value)
			value.Value = "http://localhost:" + strconv.Itoa(port)
			value.ValueClass = domain.EnvValueClassDevDefault
			value.Confidence = maxConfidence(value.Confidence, 0.82)
			value.Provenance.Source = domain.EnvValueSourceAllocator
			return
		}
	}
	if isDatabaseURLEnv(name) && hasLocalDatastoreEvidence(item, "postgres") {
		value.Value = "postgres://postgres:postgres@localhost:5432/" + envDatabaseName(item.ProjectName, repoPath)
		value.ValueClass = domain.EnvValueClassDevDefault
		value.Confidence = maxConfidence(value.Confidence, 0.86)
		value.Provenance.Source = domain.EnvValueSourceAllocator
		return
	}
	if isMongoDatabaseURLEnv(name) && hasLocalMongoDBEvidence(item) {
		value.Value = "mongodb://localhost:27017/" + envDatabaseName(item.ProjectName, repoPath)
		value.ValueClass = domain.EnvValueClassDevDefault
		value.Confidence = maxConfidence(value.Confidence, 0.86)
		value.Provenance.Source = domain.EnvValueSourceAllocator
		return
	}
	if name == "REDIS_URL" && hasTopologyServiceSignal(item, "redis") {
		port := m.assignedPort(repoPath, "redis", "redis", 6379, true)
		value.Value = "redis://localhost:" + strconv.Itoa(port)
		value.ValueClass = domain.EnvValueClassDevDefault
		value.Confidence = maxConfidence(value.Confidence, 0.86)
		value.Provenance.Source = domain.EnvValueSourceAllocator
	}
}

func (m *EnvDraftManager) assignedPortFromTargetEvidence(repoPath, targetDir, purpose string, preferred int, signals []domain.AppTopologySignal, value *domain.EnvDraftValue) int {
	exact := ""
	for _, signal := range signals {
		if signal.TargetDir == targetDir && signal.Port > 0 {
			exact = strconv.Itoa(signal.Port)
			break
		}
	}
	return m.assignedPortFromEvidence(repoPath, targetDir, purpose, preferred, exact, value)
}

func (m *EnvDraftManager) assignedPortFromEvidence(repoPath, targetDir, purpose string, preferred int, exactValue string, value *domain.EnvDraftValue) int {
	if port, ok := parsePortEvidence(exactValue); ok {
		assigned := m.assignedPort(repoPath, targetDir, purpose, port, true)
		if !m.isPortAvailable(assigned) && value != nil {
			value.Attention = append(value.Attention, fmt.Sprintf("Port %d is already in use, but repo evidence requires that exact port.", assigned))
		}
		return assigned
	}
	return m.assignedPort(repoPath, targetDir, purpose, preferred, false)
}

func (m *EnvDraftManager) assignedPort(repoPath, targetDir, purpose string, preferred int, exact bool) int {
	if m.assignedPorts == nil {
		m.assignedPorts = map[string]int{}
	}
	m.loadStoredPortAssignments(repoPath)
	key := repoPath + "\x00" + targetDir + "\x00" + purpose
	if port, ok := m.assignedPorts[key]; ok {
		return port
	}
	port := preferred
	if !exact {
		for !m.isPortAvailable(port) || m.repoPortAssigned(repoPath, key, port) {
			port++
		}
	}
	m.assignedPorts[key] = port
	m.saveStoredPortAssignment(repoPath, targetDir, purpose, port)
	return port
}

func (m *EnvDraftManager) loadStoredPortAssignments(repoPath string) {
	if m.portStore == nil || strings.TrimSpace(repoPath) == "" {
		return
	}
	if m.loadedPortRepos == nil {
		m.loadedPortRepos = map[string]bool{}
	}
	if m.loadedPortRepos[repoPath] {
		return
	}
	m.loadedPortRepos[repoPath] = true
	assignments, err := m.portStore.EnvPortAssignments(context.Background(), repoPath)
	if err != nil {
		return
	}
	for _, assignment := range assignments {
		if assignment.Port <= 0 {
			continue
		}
		key := assignment.RepoPath + "\x00" + assignment.TargetDir + "\x00" + assignment.Purpose
		m.assignedPorts[key] = assignment.Port
	}
}

func (m *EnvDraftManager) saveStoredPortAssignment(repoPath, targetDir, purpose string, port int) {
	if m.portStore == nil || strings.TrimSpace(repoPath) == "" || strings.TrimSpace(targetDir) == "" || strings.TrimSpace(purpose) == "" || port <= 0 {
		return
	}
	_ = m.portStore.SaveEnvPortAssignment(context.Background(), domain.EnvPortAssignment{
		RepoPath:  repoPath,
		TargetDir: targetDir,
		Purpose:   purpose,
		Port:      port,
	})
}

func (m *EnvDraftManager) repoPortAssigned(repoPath, currentKey string, port int) bool {
	prefix := repoPath + "\x00"
	for key, assigned := range m.assignedPorts {
		if key == currentKey || !strings.HasPrefix(key, prefix) {
			continue
		}
		if assigned == port {
			return true
		}
	}
	return false
}

func (m *EnvDraftManager) isPortAvailable(port int) bool {
	if m.portAvailable != nil {
		return m.portAvailable(port)
	}
	return isLocalPortAvailable(port)
}

func isLocalPortAvailable(port int) bool {
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

func parsePortEvidence(value string) (int, bool) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port <= 0 || port > 65535 {
		return 0, false
	}
	return port, true
}

func isBackendPortEnv(name string) bool {
	switch name {
	case "PORT", "APP_PORT", "API_PORT", "BACKEND_PORT", "SERVER_PORT":
		return true
	default:
		return false
	}
}

func isFrontendAPIURLEnv(name string) bool {
	switch name {
	case "API_URL",
		"BACKEND_URL",
		"SERVER_URL",
		"VITE_API_URL",
		"VITE_API_BASE_URL",
		"VITE_BACKEND_URL",
		"VITE_SERVER_URL",
		"NEXT_PUBLIC_API_URL",
		"NEXT_PUBLIC_API_BASE_URL",
		"NEXT_PUBLIC_BACKEND_URL",
		"NEXT_PUBLIC_SERVER_URL",
		"REACT_APP_API_URL",
		"REACT_APP_API_BASE_URL",
		"REACT_APP_BACKEND_URL",
		"REACT_APP_SERVER_URL":
		return true
	default:
		return false
	}
}

func isFrontendAppURLEnv(name string) bool {
	switch name {
	case "CLIENT_URL", "FRONTEND_URL", "WEB_URL":
		return true
	default:
		return false
	}
}

func isAppURLEnv(name string) bool {
	switch name {
	case "APP_URL", "APPLICATION_URL", "NEXT_PUBLIC_APP_URL", "VITE_APP_URL", "SITE_URL":
		return true
	default:
		return false
	}
}

func isDatabaseURLEnv(name string) bool {
	switch name {
	case "DATABASE_URL", "POSTGRES_URL":
		return true
	default:
		return false
	}
}

func isMongoDatabaseURLEnv(name string) bool {
	switch name {
	case "MONGODB_URI", "MONGO_URI", "MONGODB_URL", "MONGO_URL":
		return true
	default:
		return false
	}
}

func hasTopologyServiceSignal(item domain.EnvVarRequirement, service string) bool {
	for _, signal := range item.TopologySignals {
		if signal.Service == service {
			return true
		}
	}
	return false
}

func hasTopologySignalForDir(item domain.EnvVarRequirement, kind string) bool {
	for _, signal := range item.TopologySignals {
		if signal.Kind == kind && signal.TargetDir == item.TargetDir {
			return true
		}
	}
	return false
}

func backendTargetDir(item domain.EnvVarRequirement) (string, bool) {
	for _, signal := range item.TopologySignals {
		if signal.Kind == "backend" && signal.TargetDir != "" {
			return signal.TargetDir, true
		}
	}
	return "", false
}

func frontendTargetDir(item domain.EnvVarRequirement) (string, bool) {
	for _, signal := range item.TopologySignals {
		if signal.Kind == "frontend" && signal.TargetDir != "" && signal.TargetDir != item.TargetDir {
			return signal.TargetDir, true
		}
	}
	return "", false
}

func frontendPreferredPort(item domain.EnvVarRequirement, targetDir string) int {
	for _, signal := range item.TopologySignals {
		if signal.Kind == "port" && signal.TargetDir == targetDir && signal.Port > 0 {
			return signal.Port
		}
	}
	return 5173
}

func hasLocalMongoDBEvidence(item domain.EnvVarRequirement) bool {
	if !hasTopologyServiceSignal(item, "mongodb") {
		return false
	}
	if item.FillStrategy == "auto_fillable" {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(item.SuggestedValue))
	return strings.HasPrefix(lower, "mongodb://localhost") || strings.HasPrefix(lower, "mongodb://127.0.0.1")
}

func hasLocalDatastoreEvidence(item domain.EnvVarRequirement, service string) bool {
	if item.FillStrategy == "auto_fillable" {
		return true
	}
	for _, signal := range item.TopologySignals {
		if signal.Service == service && !strings.Contains(signal.Evidence, "env requirement") {
			return true
		}
	}
	return false
}

func topologyConfidenceForItem(item domain.EnvVarRequirement, kind string) float64 {
	for _, signal := range item.TopologySignals {
		if signal.Kind == kind && signal.TargetDir == item.TargetDir {
			return signal.Confidence
		}
	}
	return 0.5
}

func maxConfidence(a, b float64) float64 {
	if b > a {
		return b
	}
	return a
}

func envDatabaseName(projectName, repoPath string) string {
	name := strings.TrimSpace(projectName)
	if name == "" {
		name = filepath.Base(repoPath)
	}
	name = strings.ToLower(name)
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	clean := strings.Trim(builder.String(), "_")
	if clean == "" {
		return "app"
	}
	return clean
}
