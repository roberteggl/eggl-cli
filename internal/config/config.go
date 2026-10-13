package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

var (
	portMappingRE = regexp.MustCompile(`^\d+:\d+$`)
	namespaceRE   = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	resourceRE    = regexp.MustCompile(`^(?i)(svc|service|deploy|deployment|pod|sts|statefulset|daemonset|ds|job|cronjob)/[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)
)

const (
	fileName      = "config.yaml"
	MaxConfigSize = 1 << 20
)

type Config struct {
	Profiles     map[string]Profile     `yaml:"profiles"`
	PortForwards map[string]PortForward `yaml:"port_forwards"`
	Directories  map[string]string      `yaml:"directories"`
}

type Profile struct {
	KubeContext      string `yaml:"kube_context"`
	TailscaleAccount string `yaml:"tailscale_account"`
}

type PortForward struct {
	Namespace string   `yaml:"namespace"`
	Resource  string   `yaml:"resource"`
	Ports     []string `yaml:"ports"`
}

func DefaultPath() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "eggl", fileName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "eggl", fileName)
	}
	return filepath.Join(home, ".config", "eggl", fileName)
}

func Load(path string) (*Config, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if info.Size() > MaxConfigSize {
		return nil, fmt.Errorf("config: file exceeds maximum size of %d bytes", MaxConfigSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if len(c.Profiles) == 0 && len(c.Directories) == 0 {
		return fmt.Errorf("config: at least one profile or directory is required")
	}
	if err := c.validateProfiles(); err != nil {
		return err
	}
	if err := c.validateDirectories(); err != nil {
		return err
	}
	return c.validatePortForwards()
}

func (c *Config) validateProfiles() error {
	seenTargets := make(map[string]string, len(c.Profiles))
	for name, profile := range c.Profiles {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("config: profile name must not be empty")
		}
		if strings.TrimSpace(profile.KubeContext) == "" {
			return fmt.Errorf("config: profile %q: kube_context is required", name)
		}
		if err := validateKubeContext(name, profile.KubeContext); err != nil {
			return err
		}
		if strings.TrimSpace(profile.TailscaleAccount) == "" {
			return fmt.Errorf("config: profile %q: tailscale_account is required", name)
		}

		key := profileTargetKey(profile)
		if other, ok := seenTargets[key]; ok {
			return fmt.Errorf("config: profiles %q and %q share the same kube_context and tailscale_account", other, name)
		}
		seenTargets[key] = name
	}
	return nil
}

func (c *Config) validateDirectories() error {
	for name, dir := range c.Directories {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("config: directory name must not be empty")
		}
		if strings.TrimSpace(dir) == "" {
			return fmt.Errorf("config: directory %q: path is required", name)
		}
	}
	return nil
}

func (c *Config) validatePortForwards() error {
	for name, pf := range c.PortForwards {
		if err := validatePortForward(name, pf); err != nil {
			return err
		}
	}
	return nil
}

func validatePortForward(name string, pf PortForward) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("config: port_forward name must not be empty")
	}
	if strings.TrimSpace(pf.Namespace) == "" {
		return fmt.Errorf("config: port_forward %q: namespace is required", name)
	}
	if err := validateNamespace(name, pf.Namespace); err != nil {
		return err
	}
	if strings.TrimSpace(pf.Resource) == "" {
		return fmt.Errorf("config: port_forward %q: resource is required", name)
	}
	if err := validateResource(name, pf.Resource); err != nil {
		return err
	}
	for _, port := range pf.Ports {
		if err := validatePortMapping(name, port); err != nil {
			return err
		}
	}
	return nil
}

func validateKubeContext(profileName, context string) error {
	context = strings.TrimSpace(context)
	if strings.HasPrefix(context, "-") {
		return fmt.Errorf("config: profile %q: kube_context must not start with '-'", profileName)
	}
	if strings.ContainsFunc(context, unicode.IsSpace) {
		return fmt.Errorf("config: profile %q: kube_context must not contain whitespace", profileName)
	}
	if len(context) > 253 {
		return fmt.Errorf("config: profile %q: kube_context exceeds maximum length of 253", profileName)
	}
	return nil
}

func validateNamespace(pfName, namespace string) error {
	namespace = strings.TrimSpace(namespace)
	if len(namespace) > 63 {
		return fmt.Errorf("config: port_forward %q: namespace exceeds maximum length of 63", pfName)
	}
	if !namespaceRE.MatchString(namespace) {
		return fmt.Errorf("config: port_forward %q: namespace %q is invalid", pfName, namespace)
	}
	return nil
}

func validateResource(pfName, resource string) error {
	resource = strings.TrimSpace(resource)
	if !resourceRE.MatchString(resource) {
		return fmt.Errorf("config: port_forward %q: resource %q is invalid (expected kind/name, e.g. svc/my-app)", pfName, resource)
	}
	return nil
}

func validatePortMapping(pfName, port string) error {
	port = strings.TrimSpace(port)
	if !portMappingRE.MatchString(port) {
		return fmt.Errorf("config: port_forward %q: port %q must be local:remote (e.g. 8080:80)", pfName, port)
	}

	parts := strings.Split(port, ":")
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("config: port_forward %q: port %q must use ports in range 1-65535", pfName, port)
		}
	}
	return nil
}

func profileTargetKey(p Profile) string {
	return strings.TrimSpace(p.KubeContext) + "\x00" + strings.TrimSpace(p.TailscaleAccount)
}

func (c *Config) ProfileNames() []string {
	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	return names
}

func (c *Config) PortForwardNames() []string {
	names := make([]string, 0, len(c.PortForwards))
	for name := range c.PortForwards {
		names = append(names, name)
	}
	return names
}

func (c *Config) DirectoryNames() []string {
	names := make([]string, 0, len(c.Directories))
	for name := range c.Directories {
		names = append(names, name)
	}
	return names
}

func ExpandPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	if !strings.HasPrefix(path, "~") {
		return filepath.Clean(path), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expand path: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Clean(filepath.Join(home, path[2:])), nil
	}
	return "", fmt.Errorf("invalid home path %q", path)
}

func InitTemplate() string {
	return `# eggl env profiles — edit with your kubectl contexts and Tailscale account refs.
# Account refs: id from ` + "`tailscale switch --list`" + `, or tailnet slug, or account email.
#
# profiles:
#   alpha:
#     kube_context: context-a
#     tailscale_account: b3e1
#   beta:
#     kube_context: context-b
#     tailscale_account: a7f2
#
# port_forwards (optional) — use with: eggl pf <name>
#   longhorn:
#     namespace: longhorn-system
#     resource: svc/longhorn-frontend
#     ports: ["8080:80"]
#
# directories (optional) — use with: cd "$(eggl cd <name>)"
#   homelab: ~/projects/homelab
#   work: /Users/me/code/work

profiles:
  example-a:
    kube_context: context-a
    tailscale_account: account-id-a
  example-b:
    kube_context: context-b
    tailscale_account: account-id-b
`
}

func WriteInit(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if err := os.WriteFile(path, []byte(InitTemplate()), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
