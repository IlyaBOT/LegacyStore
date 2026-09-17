package macmeta

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseXMLPlist(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
	<plist version="1.0"><dict>
	<key>CFBundleName</key><string>Classic Tool</string>
	<key>CFBundleIdentifier</key><string>com.example.classic</string>
	<key>CFBundleShortVersionString</key><string>1.2.3</string>
	<key>LSMinimumSystemVersion</key><string>10.6.8</string>
	<key>LSApplicationCategoryType</key><string>public.app-category.developer-tools</string>
	</dict></plist>`)
	values, err := ParsePlist(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := plistString(values, "CFBundleName"); got != "Classic Tool" {
		t.Fatalf("name = %q", got)
	}
	if got := plistString(values, "LSMinimumSystemVersion"); got != "10.6.8" {
		t.Fatalf("minimum system = %q", got)
	}
}

func TestInspectZipApplicationBundle(t *testing.T) {
	temp := t.TempDir()
	path := filepath.Join(temp, "ClassicTool.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)

	plist := `<?xml version="1.0" encoding="UTF-8"?>
	<plist version="1.0"><dict>
	<key>CFBundleDisplayName</key><string>Classic Tool</string>
	<key>CFBundleIdentifier</key><string>com.example.classic</string>
	<key>CFBundleShortVersionString</key><string>2.4</string>
	<key>LSMinimumSystemVersion</key><string>10.5</string>
	<key>LSApplicationCategoryType</key><string>public.app-category.utilities</string>
	<key>CFBundleIconFile</key><string>Classic.icns</string>
	</dict></plist>`
	entry, err := writer.Create("Classic Tool.app/Contents/Info.plist")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(plist)); err != nil {
		t.Fatal(err)
	}

	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	iconEntry, err := writer.Create("Classic Tool.app/Contents/Resources/Classic.icns")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := iconEntry.Write(append([]byte("icns-fake-container"), pngBytes.Bytes()...)); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	result := InspectFile(context.Background(), path, "ClassicTool.zip")
	if result.Metadata.Name != "Classic Tool" {
		t.Fatalf("name = %q", result.Metadata.Name)
	}
	if result.Metadata.BundleID != "com.example.classic" {
		t.Fatalf("bundle = %q", result.Metadata.BundleID)
	}
	if result.Metadata.Version != "2.4" {
		t.Fatalf("version = %q", result.Metadata.Version)
	}
	if result.Metadata.MinimumOS != "10.5" {
		t.Fatalf("min OS = %q", result.Metadata.MinimumOS)
	}
	if result.Metadata.CategorySlug != "utilities" {
		t.Fatalf("category = %q", result.Metadata.CategorySlug)
	}
	if !strings.HasPrefix(result.Metadata.IconDataURL, "data:image/png;base64,") {
		t.Fatalf("icon data URL missing: %q", result.Metadata.IconDataURL)
	}
	if result.Metadata.IconWidth != 2 || result.Metadata.IconHeight != 2 {
		t.Fatalf("icon size = %dx%d", result.Metadata.IconWidth, result.Metadata.IconHeight)
	}
}

func TestInspectAppBundlePartsReportsMissingFields(t *testing.T) {
	result := InspectAppBundleParts([]byte(`<plist><dict><key>CFBundleName</key><string>Only Name</string></dict></plist>`), nil, "")
	if result.Metadata.Name != "Only Name" {
		t.Fatalf("name = %q", result.Metadata.Name)
	}
	codes := map[string]bool{}
	for _, warning := range result.Warnings {
		codes[warning.Code] = true
	}
	for _, expected := range []string{"missing_bundle_id", "missing_version", "missing_category", "missing_minimum_os", "missing_icon"} {
		if !codes[expected] {
			t.Fatalf("missing warning %s: %#v", expected, result.Warnings)
		}
	}
}
