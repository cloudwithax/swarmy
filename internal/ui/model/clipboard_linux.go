//go:build linux && !arm && !386 && !ios && !android

package model

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func readClipboard(f clipboardFormat) ([]byte, error) {
	switch f {
	case clipboardFormatText:
		return readTextClipboard()
	case clipboardFormatImage:
		return readImageClipboard()
	}
	return nil, errClipboardUnknownFormat
}

func readTextClipboard() ([]byte, error) {
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	if sessionType == "" {
		sessionType = "x11"
	}

	switch sessionType {
	case "wayland":
		return readClipboardWithWlPaste("text/plain")
	default:
		return readClipboardWithXclip("text/plain")
	}
}

func readImageClipboard() ([]byte, error) {
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	if sessionType == "" {
		sessionType = "x11"
	}

	var data []byte
	var err error

	switch sessionType {
	case "wayland":
		data, err = readClipboardWithWlPaste("image/png")
		if err != nil || len(data) == 0 {
			data, err = readClipboardWithWlPaste("image/")
		}
	default:
		data, err = readClipboardWithXclip("image/png")
		if err != nil || len(data) == 0 {
			data, err = readClipboardWithXclip("image/")
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read image from clipboard: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("clipboard does not contain an image")
	}

	return data, nil
}

func readClipboardWithWlPaste(mimeType string) ([]byte, error) {
	if !commandExists("wl-paste") {
		return nil, fmt.Errorf("wl-paste not found: install wl-clipboard package")
	}

	args := []string{"--no-newline"}
	if strings.Contains(mimeType, "/") {
		args = append(args, "--type", mimeType)
	}

	cmd := exec.Command("wl-paste", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("wl-paste failed: %s: %w", stderr.String(), err)
		}
		return nil, fmt.Errorf("wl-paste failed: %w", err)
	}

	return stdout.Bytes(), nil
}

func readClipboardWithXclip(mimeType string) ([]byte, error) {
	if !commandExists("xclip") {
		return readClipboardWithXsel(mimeType)
	}

	args := []string{"-selection", "clipboard"}
	if strings.Contains(mimeType, "/") {
		args = append(args, "-t", mimeType)
	}

	cmd := exec.Command("xclip", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("xclip failed: %s: %w", stderr.String(), err)
		}
		return nil, fmt.Errorf("xclip failed: %w", err)
	}

	return stdout.Bytes(), nil
}

func readClipboardWithXsel(mimeType string) ([]byte, error) {
	if !commandExists("xsel") {
		return nil, fmt.Errorf("no clipboard tool found: install xclip, xsel, or wl-clipboard")
	}

	args := []string{"--clipboard", "--output"}
	cmd := exec.Command("xsel", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("xsel failed: %s: %w", stderr.String(), err)
		}
		return nil, fmt.Errorf("xsel failed: %w", err)
	}

	return stdout.Bytes(), nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
