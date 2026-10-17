// Package paths resolves where the app stores binaries, models and chats.
package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

const appName = "LlamafileUI"

func ensure(dir string) string {
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// DataDir is ~/.local/share/LlamafileUI on Linux, ~/Library/Application
// Support/LlamafileUI on macOS and %LOCALAPPDATA%\LlamafileUI on Windows.
func DataDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return ensure(filepath.Join(home, "Library", "Application Support", appName))
	case "windows":
		if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
			return ensure(filepath.Join(dir, appName))
		}
	}
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return ensure(filepath.Join(dir, appName))
	}
	return ensure(filepath.Join(home, ".local", "share", appName))
}

func ConfigDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return DataDir()
	}
	return ensure(filepath.Join(dir, appName))
}

func BinDir() string           { return ensure(filepath.Join(DataDir(), "bin")) }
func DefaultModelsDir() string { return filepath.Join(DataDir(), "models") }
func AttachmentsDir() string   { return ensure(filepath.Join(DataDir(), "attachments")) }
func ChatDatabase() string     { return filepath.Join(DataDir(), "chats.db") }
func SettingsFile() string     { return filepath.Join(ConfigDir(), "settings.json") }
func ImagesDir() string        { return ensure(filepath.Join(DataDir(), "images")) }
func RecordingsDir() string    { return ensure(filepath.Join(DataDir(), "recordings")) }
