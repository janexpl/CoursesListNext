package pdfutil

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func RenderHTMLToPDF(ctx context.Context, pageHTML string) ([]byte, error) {
	renderCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	switch strings.ToLower(strings.TrimSpace(os.Getenv("PDF_RENDERER"))) {
	case "chrome":
		chromePath, err := findChromeExecutable()
		if err != nil {
			return nil, err
		}
		return renderPDFWithChrome(renderCtx, chromePath, pageHTML)
	case "wkhtmltopdf", "":
		return renderPDFWithWkhtmltopdf(renderCtx, pageHTML)
	default:
		return nil, fmt.Errorf("unsupported PDF_RENDERER value")
	}
}

func renderPDFWithChrome(ctx context.Context, chromePath, pageHTML string) ([]byte, error) {
	htmlFile, err := os.CreateTemp("", "document-*.html")
	if err != nil {
		return nil, err
	}
	defer os.Remove(htmlFile.Name())

	if _, err := htmlFile.WriteString(pageHTML); err != nil {
		htmlFile.Close()
		return nil, err
	}
	if err := htmlFile.Close(); err != nil {
		return nil, err
	}

	pdfFile, err := os.CreateTemp("", "document-*.pdf")
	if err != nil {
		return nil, err
	}
	pdfPath := pdfFile.Name()
	if err := pdfFile.Close(); err != nil {
		return nil, err
	}
	defer os.Remove(pdfPath)

	userDataDir, err := os.MkdirTemp("", "document-chrome-profile-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(userDataDir)

	htmlURL := fileURL(htmlFile.Name())
	cmd := exec.CommandContext(
		ctx,
		chromePath,
		"--headless",
		"--disable-gpu",
		"--no-first-run",
		"--no-default-browser-check",
		"--allow-file-access-from-files",
		"--disable-dev-shm-usage",
		"--user-data-dir="+userDataDir,
		"--print-to-pdf="+pdfPath,
		"--no-pdf-header-footer",
		htmlURL,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("chrome pdf failed: %s", strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}

	return os.ReadFile(pdfPath)
}

func renderPDFWithWkhtmltopdf(ctx context.Context, pageHTML string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "wkhtmltopdf", "--quiet", "-", "-")
	cmd.Stdin = strings.NewReader(pageHTML)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("wkhtmltopdf failed: %s", strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}

	return stdout.Bytes(), nil
}

func findChromeExecutable() (string, error) {
	if value := strings.TrimSpace(os.Getenv("CHROME_BIN")); value != "" {
		if _, err := os.Stat(value); err == nil {
			return value, nil
		}
		if resolved, err := exec.LookPath(value); err == nil {
			return resolved, nil
		}
	}

	if value := strings.TrimSpace(os.Getenv("CHROME_PATH")); value != "" {
		if _, err := os.Stat(value); err == nil {
			return value, nil
		}
		if resolved, err := exec.LookPath(value); err == nil {
			return resolved, nil
		}
	}

	candidates := []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"chrome",
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
	}

	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		)
	case "windows":
		programFiles := os.Getenv("PROGRAMFILES")
		programFilesX86 := os.Getenv("PROGRAMFILES(X86)")
		localAppData := os.Getenv("LOCALAPPDATA")
		candidates = append(candidates,
			filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFiles, "Chromium", "Application", "chrome.exe"),
			filepath.Join(programFilesX86, "Chromium", "Application", "chrome.exe"),
		)
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved, nil
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("chrome executable not found")
}

func fileURL(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}

	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}).String()
}
