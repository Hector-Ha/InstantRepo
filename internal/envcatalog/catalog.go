package envcatalog

import (
	"fmt"
	"strings"

	"instantrepo/internal/domain"
)

const (
	ActionGenerateLocalSecret = "generate_local_secret"
	ActionFillDevDefault      = "fill_dev_default"
	ActionLeaveBlank          = "leave_blank"
	ActionShowAttention       = "show_attention"
)

type Catalog struct {
	Version        string
	AllowedActions []string
	Rules          []Rule
}

type Rule struct {
	Names        []string
	Suffixes     []string
	Action       string
	ValueClass   string
	DefaultValue string
	Confidence   float64
	Instructions []string
	Attention    []string
}

type Decision struct {
	Action       string
	ValueClass   string
	DefaultValue string
	Confidence   float64
	Instructions []string
	Attention    []string
}

func DefaultCatalog() Catalog {
	return Catalog{
		Version: "2026.05.18-foundation",
		AllowedActions: []string{
			ActionGenerateLocalSecret,
			ActionFillDevDefault,
			ActionLeaveBlank,
			ActionShowAttention,
		},
		Rules: []Rule{
			{
				Names:      []string{"JWT_SECRET", "AUTH_SECRET", "NEXTAUTH_SECRET", "SESSION_SECRET", "COOKIE_SECRET", "CSRF_SECRET"},
				Suffixes:   []string{"_JWT_SECRET", "_AUTH_SECRET", "_SESSION_SECRET"},
				Action:     ActionGenerateLocalSecret,
				ValueClass: domain.EnvValueClassGeneratedLocalSecret,
				Confidence: 0.94,
				Instructions: []string{
					"Generated for local development. Replace it in production outside InstantRepo.",
				},
			},
			{
				Names:        []string{"DATABASE_URL", "POSTGRES_URL"},
				Action:       ActionFillDevDefault,
				ValueClass:   domain.EnvValueClassDevDefault,
				DefaultValue: "postgres://postgres:postgres@localhost:5432/postgres",
				Confidence:   0.78,
				Instructions: []string{
					"Uses a conventional local PostgreSQL URL for development.",
				},
			},
			{
				Names: []string{
					"SUPABASE_URL",
					"SUPABASE_ANON_KEY",
					"FIREBASE_API_KEY",
					"FIREBASE_AUTH_DOMAIN",
					"FIREBASE_PROJECT_ID",
					"VITE_SUPABASE_URL",
					"VITE_SUPABASE_ANON_KEY",
				},
				Action:     ActionLeaveBlank,
				ValueClass: domain.EnvValueClassProviderConfig,
				Confidence: 0.86,
				Instructions: []string{
					"Use project-specific provider configuration from the service dashboard.",
				},
			},
			{
				Names: []string{
					"OPENAI_API_KEY",
					"STRIPE_SECRET_KEY",
					"STRIPE_API_KEY",
					"SENDGRID_API_KEY",
					"CLERK_SECRET_KEY",
				},
				Suffixes:   []string{"_API_KEY", "_ACCESS_TOKEN"},
				Action:     ActionLeaveBlank,
				ValueClass: domain.EnvValueClassServiceCredential,
				Confidence: 0.88,
				Instructions: []string{
					"Use a real service credential for local development. InstantRepo does not invent provider keys.",
				},
			},
		},
	}
}

func (c Catalog) Validate() error {
	allowed := map[string]bool{}
	for _, action := range c.AllowedActions {
		allowed[action] = true
	}
	for _, rule := range c.Rules {
		if strings.TrimSpace(rule.Action) == "" {
			return fmt.Errorf("catalog rule action is required")
		}
		if !allowed[rule.Action] {
			return fmt.Errorf("catalog action %q is not allowed", rule.Action)
		}
	}
	return nil
}

func (c Catalog) Classify(name string) (Decision, bool) {
	upperName := strings.ToUpper(strings.TrimSpace(name))
	for _, rule := range c.Rules {
		if rule.matches(upperName) {
			return Decision{
				Action:       rule.Action,
				ValueClass:   rule.ValueClass,
				DefaultValue: rule.DefaultValue,
				Confidence:   rule.Confidence,
				Instructions: append([]string(nil), rule.Instructions...),
				Attention:    append([]string(nil), rule.Attention...),
			}, true
		}
	}
	return Decision{}, false
}

func (r Rule) matches(upperName string) bool {
	for _, name := range r.Names {
		if strings.EqualFold(name, upperName) {
			return true
		}
	}
	for _, suffix := range r.Suffixes {
		if strings.HasSuffix(upperName, strings.ToUpper(suffix)) {
			return true
		}
	}
	return false
}
