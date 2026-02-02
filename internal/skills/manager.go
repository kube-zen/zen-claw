package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Skill struct {
	Name        string
	Description string
	Location    string
	Enabled     bool
}

type Manager struct {
	skillsDir string
	skills    map[string]Skill
}

func NewManager(skillsDir string) (*Manager, error) {
	mgr := &Manager{
		skillsDir: skillsDir,
		skills:    make(map[string]Skill),
	}

	// Load skills
	if err := mgr.loadSkills(); err != nil {
		return nil, err
	}

	return mgr, nil
}

func (m *Manager) loadSkills() error {
	// Create skills directory if it doesn't exist
	if err := os.MkdirAll(m.skillsDir, 0755); err != nil {
		return fmt.Errorf("create skills directory: %w", err)
	}

	// Look for skill directories
	entries, err := os.ReadDir(m.skillsDir)
	if err != nil {
		return fmt.Errorf("read skills directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		skillPath := filepath.Join(m.skillsDir, skillName)

		// Check for SKILL.md file
		skillFile := filepath.Join(skillPath, "SKILL.md")
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			continue // Not a valid skill
		}

		// Read skill description
		desc := m.readSkillDescription(skillFile)

		m.skills[skillName] = Skill{
			Name:        skillName,
			Description: desc,
			Location:    skillPath,
			Enabled:     true,
		}
	}

	return nil
}

func (m *Manager) readSkillDescription(skillFile string) string {
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return "No description available"
	}

	content := string(data)
	// Extract first line or first paragraph as description
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			// Return first non-empty, non-header line
			if len(line) > 100 {
				return line[:100] + "..."
			}
			return line
		}
	}

	return "No description"
}

func (m *Manager) List() []Skill {
	var skills []Skill
	for _, skill := range m.skills {
		skills = append(skills, skill)
	}
	return skills
}

func (m *Manager) Get(name string) (Skill, bool) {
	skill, ok := m.skills[name]
	return skill, ok
}

func (m *Manager) LoadSkill(name string) (string, error) {
	skill, ok := m.skills[name]
	if !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}

	skillFile := filepath.Join(skill.Location, "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return "", fmt.Errorf("read skill file: %w", err)
	}

	return string(data), nil
}

func (m *Manager) CreateSkill(name, description string) error {
	skillPath := filepath.Join(m.skillsDir, name)
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		return fmt.Errorf("create skill directory: %w", err)
	}

	skillFile := filepath.Join(skillPath, "SKILL.md")
	content := fmt.Sprintf("# %s\n\n%s\n\n## Usage\n\nAdd usage instructions here.\n\n## Examples\n\n```bash\n# Example commands\n```\n", name, description)

	if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("write skill file: %w", err)
	}

	// Reload skills
	return m.loadSkills()
}
