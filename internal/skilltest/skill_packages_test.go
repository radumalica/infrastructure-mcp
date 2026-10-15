package skilltest

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var (
	skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	markdownLink     = regexp.MustCompile(`\]\((\./[^)]+)\)`)
)

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type evaluationSuite struct {
	Skills []skillEvaluation `yaml:"skills"`
}

type skillEvaluation struct {
	Name            string   `yaml:"name"`
	PositivePrompts []string `yaml:"positive_prompts"`
	NegativePrompts []string `yaml:"negative_prompts"`
	RequiredPhrases []string `yaml:"required_phrases"`
}

func TestSkillPackages(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	skillsRoot := filepath.Join(repoRoot, ".github", "skills")
	expected := []string{"infrastructure-mcp-operator"}

	validatePublicSkillSet(t, skillsRoot, expected)

	for _, name := range expected {
		t.Run(name, func(t *testing.T) {
			validateSkillPackage(t, skillsRoot, name)
		})
	}

	validateEvaluationFixtures(t, repoRoot, skillsRoot, expected)
}

func validatePublicSkillSet(t *testing.T, skillsRoot string, expected []string) {
	t.Helper()
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		t.Fatalf("read public skills directory: %v", err)
	}

	actual := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			actual = append(actual, entry.Name())
		}
	}
	if strings.Join(actual, ",") != strings.Join(expected, ",") {
		t.Fatalf("public skills = %v, want exactly %v; premium skills must live outside this repository", actual, expected)
	}
}

func validateSkillPackage(t *testing.T, skillsRoot, name string) {
	t.Helper()
	dir := filepath.Join(skillsRoot, name)
	expectedFiles := map[string]bool{
		"SKILL.md":               false,
		"references/workflow.md": false,
	}
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}

	frontmatter := parseFrontmatter(t, data)
	if frontmatter.Name != name {
		t.Errorf("frontmatter name = %q, want folder name %q", frontmatter.Name, name)
	}
	if !skillNamePattern.MatchString(frontmatter.Name) {
		t.Errorf("frontmatter name %q is not a lowercase hyphenated slug", frontmatter.Name)
	}
	if !strings.Contains(frontmatter.Description, "Use when") {
		t.Errorf("description must include trigger guidance with 'Use when'")
	}

	links := markdownLink.FindAllSubmatch(data, -1)
	if len(links) == 0 {
		t.Fatal("SKILL.md must reference at least one local supporting file")
	}
	for _, match := range links {
		relative := string(match[1])
		if strings.Contains(relative, "..") {
			t.Errorf("supporting file reference escapes skill package: %s", relative)
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(relative, "./")))); err != nil {
			t.Errorf("supporting file %s: %v", relative, err)
		}
	}

	err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Errorf("symlinks are not allowed: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if _, ok := expectedFiles[relative]; !ok {
			t.Errorf("unexpected public skill file: %s", relative)
		} else {
			expectedFiles[relative] = true
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.IndexByte(contents, 0) >= 0 {
			t.Errorf("binary file is not allowed: %s", path)
		}
		for _, forbidden := range []string{"BEGIN PRIVATE KEY", "AKIA", "ghp_"} {
			if bytes.Contains(contents, []byte(forbidden)) {
				t.Errorf("possible embedded secret marker %q in %s", forbidden, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk package: %v", err)
	}
	for path, found := range expectedFiles {
		if !found {
			t.Errorf("missing required public skill file: %s", path)
		}
	}
}

func parseFrontmatter(t *testing.T, data []byte) skillFrontmatter {
	t.Helper()
	parts := bytes.SplitN(data, []byte("---"), 3)
	if len(parts) != 3 || len(bytes.TrimSpace(parts[0])) != 0 {
		t.Fatal("SKILL.md must start with YAML frontmatter")
	}
	var frontmatter skillFrontmatter
	if err := yaml.Unmarshal(parts[1], &frontmatter); err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	if frontmatter.Name == "" || frontmatter.Description == "" {
		t.Fatal("frontmatter requires name and description")
	}
	return frontmatter
}

// assertRequiredPhrases verifies each fixture phrase appears verbatim in the
// skill's SKILL.md, ignoring line wrapping, so the safety contract cannot be
// edited out without failing CI.
func assertRequiredPhrases(t *testing.T, skillsRoot, name string, phrases []string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(skillsRoot, name, "SKILL.md"))
	if err != nil {
		t.Errorf("read SKILL.md for %s: %v", name, err)
		return
	}
	body := strings.Join(strings.Fields(string(data)), " ")
	for _, phrase := range phrases {
		if !strings.Contains(body, strings.Join(strings.Fields(phrase), " ")) {
			t.Errorf("%s SKILL.md is missing required phrase %q", name, phrase)
		}
	}
}

func validateEvaluationFixtures(t *testing.T, repoRoot, skillsRoot string, expected []string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, "internal", "skilltest", "testdata", "evaluations.yaml"))
	if err != nil {
		t.Fatalf("read evaluation fixtures: %v", err)
	}
	var suite evaluationSuite
	if err := yaml.Unmarshal(data, &suite); err != nil {
		t.Fatalf("parse evaluation fixtures: %v", err)
	}

	byName := make(map[string]skillEvaluation, len(suite.Skills))
	for _, evaluation := range suite.Skills {
		if _, exists := byName[evaluation.Name]; exists {
			t.Errorf("duplicate evaluation fixture for %s", evaluation.Name)
			continue
		}
		byName[evaluation.Name] = evaluation
	}
	if len(byName) != len(expected) {
		t.Errorf("evaluation fixtures contain %d skills, want exactly %d public skills", len(byName), len(expected))
	}
	for _, name := range expected {
		evaluation, ok := byName[name]
		if !ok {
			t.Errorf("missing evaluation fixture for %s", name)
			continue
		}
		if len(evaluation.PositivePrompts) < 5 {
			t.Errorf("%s has %d positive prompts, want at least 5", name, len(evaluation.PositivePrompts))
		}
		if len(evaluation.NegativePrompts) < 5 {
			t.Errorf("%s has %d negative prompts, want at least 5", name, len(evaluation.NegativePrompts))
		}
		if len(evaluation.RequiredPhrases) == 0 {
			t.Errorf("%s has no required safety or sequencing assertions", name)
		}
		assertRequiredPhrases(t, skillsRoot, name, evaluation.RequiredPhrases)
	}
}
