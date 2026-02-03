package plugins

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/neves/zen-claw/internal/agent"
)

// DefaultPluginDir returns the default plugin directory
func DefaultPluginDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".zen", "zen-claw", "plugins")
}

// Loader manages plugin discovery and loading
type Loader struct {
	pluginDirs []string
	plugins    map[string]*Plugin
}

// NewLoader creates a new plugin loader
func NewLoader(dirs ...string) *Loader {
	if len(dirs) == 0 {
		dirs = []string{DefaultPluginDir()}
	}
	return &Loader{
		pluginDirs: dirs,
		plugins:    make(map[string]*Plugin),
	}
}

// LoadAll discovers and loads all plugins from configured directories
func (l *Loader) LoadAll() error {
	for _, dir := range l.pluginDirs {
		if err := l.loadFromDir(dir); err != nil {
			log.Printf("[Plugins] Warning: failed to load from %s: %v", dir, err)
		}
	}
	return nil
}

// loadFromDir loads plugins from a single directory
func (l *Loader) loadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No plugins directory yet
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(dir, entry.Name())
		manifestPath := filepath.Join(pluginDir, "plugin.yaml")

		// Skip directories without plugin.yaml
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			continue
		}

		plugin, err := LoadPlugin(pluginDir)
		if err != nil {
			log.Printf("[Plugins] Failed to load %s: %v", entry.Name(), err)
			continue
		}

		l.plugins[plugin.Manifest.Name] = plugin
		log.Printf("[Plugins] Loaded: %s v%s - %s",
			plugin.Manifest.Name,
			plugin.Manifest.Version,
			plugin.Manifest.Description)
	}

	return nil
}

// GetPlugin returns a plugin by name
func (l *Loader) GetPlugin(name string) (*Plugin, bool) {
	p, ok := l.plugins[name]
	return p, ok
}

// GetTools returns all loaded plugins as agent.Tool slice
func (l *Loader) GetTools() []agent.Tool {
	tools := make([]agent.Tool, 0, len(l.plugins))
	for _, p := range l.plugins {
		tools = append(tools, p.ToTool())
	}
	return tools
}

// ListPlugins returns info about all loaded plugins
func (l *Loader) ListPlugins() []PluginInfo {
	infos := make([]PluginInfo, 0, len(l.plugins))
	for _, p := range l.plugins {
		infos = append(infos, PluginInfo{
			Name:        p.Manifest.Name,
			Version:     p.Manifest.Version,
			Description: p.Manifest.Description,
			Author:      p.Manifest.Author,
			Dir:         p.Dir,
		})
	}
	return infos
}

// PluginInfo contains basic plugin information
type PluginInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author,omitempty"`
	Dir         string `json:"dir"`
}

// Count returns the number of loaded plugins
func (l *Loader) Count() int {
	return len(l.plugins)
}

// Reload reloads all plugins
func (l *Loader) Reload() error {
	l.plugins = make(map[string]*Plugin)
	return l.LoadAll()
}

// LoadSingle loads a single plugin from a directory
func (l *Loader) LoadSingle(dir string) error {
	plugin, err := LoadPlugin(dir)
	if err != nil {
		return fmt.Errorf("load plugin: %w", err)
	}

	l.plugins[plugin.Manifest.Name] = plugin
	log.Printf("[Plugins] Loaded: %s v%s", plugin.Manifest.Name, plugin.Manifest.Version)
	return nil
}
