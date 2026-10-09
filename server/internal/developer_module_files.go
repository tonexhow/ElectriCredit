package internal

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func resolveModuleSourceDir() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("ELECTRICREDIT_MODULE_DIR")); configured != "" {
		return filepath.Abs(configured)
	}
	candidates := []string{filepath.Join("..", "module"), "module"}
	for _, candidate := range candidates {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absolute); err == nil && info.IsDir() {
			return absolute, nil
		}
	}
	return filepath.Abs(filepath.Join("..", "module"))
}

func moduleSourcePaths(kind string) (readme, guide, code string, ok bool) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "hub":
		return "README.md", filepath.Join("hub", "hub_instruction.md"), filepath.Join("hub", "hub.ino"), true
	case "registry":
		return "README.md", filepath.Join("registry", "registry_instruction.md"), filepath.Join("registry", "registry.ino"), true
	default:
		return "", "", "", false
	}
}

func (api *API) handleDeveloperModuleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, ok := api.requireDeveloper(w, r); !ok {
		return
	}
	kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kind")))
	readmeRel, guideRel, _, valid := moduleSourcePaths(kind)
	if !valid {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "choose Hub or Registry"})
		return
	}
	base, err := resolveModuleSourceDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "module source folder is unavailable"})
		return
	}
	readme, err := os.ReadFile(filepath.Join(base, readmeRel))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "module README was not found"})
		return
	}
	guide, err := os.ReadFile(filepath.Join(base, guideRel))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "module instruction file was not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"kind":         kind,
		"readme_name":  "module/README.md",
		"guide_name":   "module/" + kind + "/" + kind + "_instruction.md",
		"code_name":    "module/" + kind + "/" + kind + ".ino",
		"library_name": "module/libraries/ElectriCreditShared.zip",
		"readme":       string(readme),
		"guide":        string(guide),
	})
}

func (api *API) handleDeveloperModuleCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, ok := api.requireDeveloper(w, r); !ok {
		return
	}
	kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kind")))
	_, _, codeRel, valid := moduleSourcePaths(kind)
	if !valid {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "choose Hub or Registry"})
		return
	}
	base, err := resolveModuleSourceDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "module source folder is unavailable"})
		return
	}
	path := filepath.Join(base, codeRel)
	if _, err := os.Stat(path); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "module firmware source was not found"})
		return
	}
	w.Header().Set("Content-Type", "text/x-arduino; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+kind+`.ino"`)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, path)
}

func (api *API) handleDeveloperModuleLibrary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, ok := api.requireDeveloper(w, r); !ok {
		return
	}
	base, err := resolveModuleSourceDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "module source folder is unavailable"})
		return
	}
	path := filepath.Join(base, "libraries", "ElectriCreditShared.zip")
	if _, err := os.Stat(path); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ElectriCreditShared library ZIP was not found"})
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="ElectriCreditShared.zip"`)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, path)
}
