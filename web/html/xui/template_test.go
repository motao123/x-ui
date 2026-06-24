package xui_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestAntDesignVueTemplatesDoNotUseSelfClosingComponents(t *testing.T) {
	pattern := regexp.MustCompile(`<a-[a-zA-Z0-9-]+[^>]*\/>`)
	root := "."
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".html" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if match := pattern.Find(content); len(match) > 0 {
			t.Fatalf("%s contains self-closing Ant Design Vue component: %s", path, match)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
