package macmeta

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Metadata struct {
	Name         string `json:"name,omitempty"`
	BundleID     string `json:"bundle_id,omitempty"`
	Version      string `json:"version,omitempty"`
	CategorySlug string `json:"category_slug,omitempty"`
	MinimumOS    string `json:"minimum_os,omitempty"`
	IconDataURL  string `json:"icon_data_url,omitempty"`
	IconWidth    int    `json:"icon_width,omitempty"`
	IconHeight   int    `json:"icon_height,omitempty"`
}

type Result struct {
	FileName    string    `json:"file_name"`
	PackageType string    `json:"package_type"`
	Metadata    Metadata  `json:"metadata"`
	Warnings    []Warning `json:"warnings,omitempty"`
}

var categoryMap = map[string]string{
	"public.app-category.books":                "books",
	"public.app-category.business":             "business",
	"public.app-category.developer-tools":      "developer-tools",
	"public.app-category.education":            "education",
	"public.app-category.entertainment":        "entertainment",
	"public.app-category.finance":              "finance",
	"public.app-category.food-drink":           "food-drink",
	"public.app-category.games":                "games",
	"public.app-category.graphics-design":      "graphics-design",
	"public.app-category.healthcare-fitness":   "health-fitness",
	"public.app-category.lifestyle":            "lifestyle",
	"public.app-category.magazines-newspapers": "magazines-newspapers",
	"public.app-category.medical":              "medical",
	"public.app-category.music":                "music",
	"public.app-category.navigation":           "navigation",
	"public.app-category.news":                 "news",
	"public.app-category.photography":          "photo-video",
	"public.app-category.photo-video":          "photo-video",
	"public.app-category.productivity":         "productivity",
	"public.app-category.reference":            "reference",
	"public.app-category.safari-extensions":    "safari-extensions",
	"public.app-category.shopping":             "shopping",
	"public.app-category.social-networking":    "social-networking",
	"public.app-category.sports":               "sports",
	"public.app-category.travel":               "travel",
	"public.app-category.utilities":            "utilities",
	"public.app-category.video":                "photo-video",
	"public.app-category.weather":              "weather",
}

func InspectFile(ctx context.Context, path, originalName string) Result {
	result := Result{
		FileName:    filepath.Base(strings.TrimSpace(originalName)),
		PackageType: packageType(originalName),
	}
	switch result.PackageType {
	case "zip", "app":
		if err := inspectZip(path, &result); err != nil {
			result.addWarning("metadata_parse_failed", "Не удалось прочитать метаданные пакета: "+err.Error())
		}
	case "dmg", "pkg":
		if err := inspectWith7Zip(ctx, path, &result); err != nil {
			result.addWarning("metadata_parse_failed", "Не удалось автоматически разобрать "+strings.ToUpper(result.PackageType)+": "+err.Error())
		}
	default:
		result.addWarning("unsupported_metadata_format", "Автоматическое чтение метаданных для этого типа файла не поддерживается.")
	}
	result.applyFallbacks()
	result.validateMetadata()
	return result
}

func InspectAppBundleParts(plistData, iconData []byte, iconName string) Result {
	result := Result{FileName: "Dropped.app", PackageType: "app"}
	values, err := ParsePlist(plistData)
	if err != nil {
		result.addWarning("invalid_info_plist", "Info.plist приложения повреждён или имеет неизвестный формат.")
		result.applyFallbacks()
		result.validateMetadata()
		return result
	}
	applyPlist(values, &result.Metadata)
	if len(iconData) > 0 {
		if dataURL, width, height, err := iconDataURL(iconData, iconName); err == nil {
			result.Metadata.IconDataURL = dataURL
			result.Metadata.IconWidth = width
			result.Metadata.IconHeight = height
		} else {
			result.addWarning("icon_parse_failed", "Иконка приложения найдена, но её не удалось декодировать.")
		}
	}
	result.validateMetadata()
	return result
}

func inspectZip(path string, result *Result) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()

	var plistFile *zip.File
	for _, file := range reader.File {
		lower := strings.ToLower(filepath.ToSlash(file.Name))
		if strings.HasSuffix(lower, ".app/contents/info.plist") || lower == "contents/info.plist" {
			plistFile = file
			break
		}
	}
	if plistFile == nil {
		return errors.New("Contents/Info.plist не найден")
	}
	plistData, err := readZipFile(plistFile, 8<<20)
	if err != nil {
		return err
	}
	values, err := ParsePlist(plistData)
	if err != nil {
		return err
	}
	applyPlist(values, &result.Metadata)

	iconName := plistString(values, "CFBundleIconFile", "CFBundleIconName")
	if iconName != "" {
		prefix := filepath.ToSlash(plistFile.Name)
		prefix = strings.TrimSuffix(prefix, "Info.plist") + "Resources/"
		iconFile := findZipIcon(reader.File, prefix, iconName)
		if iconFile != nil {
			data, readErr := readZipFile(iconFile, 16<<20)
			if readErr == nil {
				if dataURL, width, height, iconErr := iconDataURL(data, iconFile.Name); iconErr == nil {
					result.Metadata.IconDataURL = dataURL
					result.Metadata.IconWidth = width
					result.Metadata.IconHeight = height
				}
			}
		}
	}
	return nil
}

type extractionCandidate struct {
	path  string
	depth int
}

func inspectWith7Zip(ctx context.Context, path string, result *Result) error {
	sevenZip, err := find7Zip()
	if err != nil {
		return err
	}

	workDir, err := os.MkdirTemp("", "legacystore-inspect-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	queue := []extractionCandidate{{path: path, depth: 0}}
	visited := make(map[string]bool)
	processed := 0
	var lastErr error

	for len(queue) > 0 && processed < 16 {
		candidate := queue[0]
		queue = queue[1:]
		if candidate.depth > 4 {
			continue
		}
		absolute, absErr := filepath.Abs(candidate.path)
		if absErr == nil {
			if visited[absolute] {
				continue
			}
			visited[absolute] = true
		}

		stageDir := filepath.Join(workDir, fmt.Sprintf("stage-%02d", processed))
		processed++
		if mkErr := os.MkdirAll(stageDir, 0750); mkErr != nil {
			lastErr = mkErr
			continue
		}
		if extractErr := extractWith7Zip(ctx, sevenZip, candidate.path, stageDir); extractErr != nil {
			lastErr = extractErr
			if candidate.depth == 0 {
				return extractErr
			}
			continue
		}

		if inspectErr := inspectExtractedTree(stageDir, result); inspectErr == nil {
			return nil
		} else {
			lastErr = inspectErr
		}

		// Flat PKGs can expose useful metadata before their Payload is unpacked.
		// Keep those values, but continue recursively because the application
		// bundle usually lives inside Payload.
		_ = inspectPackageXML(stageDir, result)

		if candidate.depth < 4 {
			nested, nestedErr := nestedExtractionCandidates(stageDir, candidate.depth+1)
			if nestedErr == nil {
				queue = append(queue, nested...)
			}
		}
	}

	if lastErr != nil {
		return fmt.Errorf("метаданные приложения внутри образа не найдены: %w", lastErr)
	}
	return errors.New("метаданные приложения внутри образа не найдены")
}

func find7Zip() (string, error) {
	for _, name := range []string{"7zz", "7z"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("7-Zip (7zz/7z) недоступен на сервере")
}

func extractWith7Zip(ctx context.Context, sevenZip, source, destination string) error {
	commandContext, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	command := exec.CommandContext(commandContext, sevenZip, "x", "-y", "-bd", "-bb0", "-o"+destination, source)
	output, err := command.CombinedOutput()
	if commandContext.Err() != nil {
		return errors.New("разбор пакета превысил лимит времени")
	}
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(output))
	if len(message) > 180 {
		message = message[:180]
	}
	if message == "" {
		message = err.Error()
	}
	return fmt.Errorf("7z: %s", message)
}

func nestedExtractionCandidates(root string, depth int) ([]extractionCandidate, error) {
	candidates := make([]extractionCandidate, 0, 4)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if len(candidates) >= 12 {
			return nil
		}
		if isNestedArchiveCandidate(path) {
			candidates = append(candidates, extractionCandidate{path: path, depth: depth})
		}
		return nil
	})
	return candidates, err
}

func isNestedArchiveCandidate(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(base))
	switch ext {
	case ".dmg", ".hfs", ".hfsx", ".img", ".image", ".apfs", ".pkg", ".mpkg", ".xar", ".cpio", ".gz", ".bz2", ".xz", ".lzma":
		return true
	}
	if base == "payload" || strings.HasPrefix(base, "payload~") {
		return true
	}
	return hasDiskOrArchiveSignature(path)
}

func hasDiskOrArchiveSignature(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	buffer := make([]byte, 4096)
	n, _ := file.Read(buffer)
	buffer = buffer[:n]
	if len(buffer) >= 1026 {
		magic := string(buffer[1024:1026])
		if magic == "H+" || magic == "HX" {
			return true
		}
	}
	if len(buffer) >= 520 && string(buffer[512:520]) == "EFI PART" {
		return true
	}
	if len(buffer) >= 36 && string(buffer[32:36]) == "NXSB" {
		return true
	}
	if bytes.HasPrefix(buffer, []byte("xar!")) || bytes.HasPrefix(buffer, []byte("070701")) || bytes.HasPrefix(buffer, []byte("070702")) {
		return true
	}
	if len(buffer) >= 2 && buffer[0] == 0x1f && buffer[1] == 0x8b {
		return true
	}
	return false
}

func inspectExtractedTree(root string, result *Result) error {
	var plistPath string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || plistPath != "" || entry.IsDir() {
			return nil
		}
		normalized := strings.ToLower(filepath.ToSlash(path))
		if strings.HasSuffix(normalized, ".app/contents/info.plist") {
			plistPath = path
		}
		return nil
	})
	if err != nil {
		return err
	}
	if plistPath == "" {
		return errors.New("Info.plist не найден")
	}
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return err
	}
	values, err := ParsePlist(data)
	if err != nil {
		return err
	}
	applyPlist(values, &result.Metadata)

	iconName := plistString(values, "CFBundleIconFile", "CFBundleIconName")
	if iconName == "" {
		return nil
	}
	resources := filepath.Join(filepath.Dir(plistPath), "Resources")
	iconPath := findIconOnDisk(resources, iconName)
	if iconPath == "" {
		return nil
	}
	iconBytes, err := os.ReadFile(iconPath)
	if err != nil {
		return nil
	}
	dataURL, width, height, err := iconDataURL(iconBytes, iconPath)
	if err != nil {
		return nil
	}
	result.Metadata.IconDataURL = dataURL
	result.Metadata.IconWidth = width
	result.Metadata.IconHeight = height
	return nil
}

func inspectPackageXML(root string, result *Result) error {
	var packageInfo string
	var distribution string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Base(path)) {
		case "packageinfo":
			if packageInfo == "" {
				packageInfo = path
			}
		case "distribution":
			if distribution == "" {
				distribution = path
			}
		}
		return nil
	})
	if packageInfo == "" && distribution == "" {
		return errors.New("PackageInfo не найден")
	}
	if packageInfo != "" {
		data, err := os.ReadFile(packageInfo)
		if err == nil {
			var info struct {
				XMLName    xml.Name `xml:"pkg-info"`
				Identifier string   `xml:"identifier,attr"`
				Version    string   `xml:"version,attr"`
			}
			if xml.Unmarshal(data, &info) == nil {
				result.Metadata.BundleID = strings.TrimSpace(info.Identifier)
				result.Metadata.Version = strings.TrimSpace(info.Version)
			}
		}
	}
	if distribution != "" {
		data, err := os.ReadFile(distribution)
		if err == nil {
			var doc struct {
				Title string `xml:"title"`
			}
			if xml.Unmarshal(data, &doc) == nil {
				result.Metadata.Name = strings.TrimSpace(doc.Title)
			}
		}
	}
	return nil
}

func applyPlist(values map[string]any, metadata *Metadata) {
	metadata.Name = plistString(values, "CFBundleDisplayName", "CFBundleName")
	metadata.BundleID = plistString(values, "CFBundleIdentifier")
	metadata.Version = plistString(values, "CFBundleShortVersionString", "CFBundleVersion")
	metadata.MinimumOS = plistString(values, "LSMinimumSystemVersion", "MinimumOSVersion")
	category := strings.ToLower(plistString(values, "LSApplicationCategoryType"))
	if mapped, ok := categoryMap[category]; ok {
		metadata.CategorySlug = mapped
	}
}

func (result *Result) applyFallbacks() {
	if result.Metadata.Name == "" {
		base := strings.TrimSuffix(result.FileName, filepath.Ext(result.FileName))
		base = strings.TrimSpace(strings.ReplaceAll(base, "_", " "))
		base = strings.TrimSpace(strings.ReplaceAll(base, "-", " "))
		if base != "" {
			result.Metadata.Name = base
			result.addWarning("name_from_filename", "Название приложения не найдено в метаданных и взято из имени файла.")
		}
	}
}

func (result *Result) validateMetadata() {
	if strings.TrimSpace(result.Metadata.Name) == "" {
		result.addWarning("missing_name", "Не удалось определить название приложения.")
	}
	if strings.TrimSpace(result.Metadata.BundleID) == "" {
		result.addWarning("missing_bundle_id", "Не удалось определить Bundle ID.")
	}
	if strings.TrimSpace(result.Metadata.Version) == "" {
		result.addWarning("missing_version", "Не удалось определить версию приложения.")
	}
	if strings.TrimSpace(result.Metadata.CategorySlug) == "" {
		result.addWarning("missing_category", "Категория приложения не указана или имеет неизвестное значение.")
	}
	if strings.TrimSpace(result.Metadata.MinimumOS) == "" {
		result.addWarning("missing_minimum_os", "Минимальная версия OS X/macOS не указана в метаданных.")
	}
	if strings.TrimSpace(result.Metadata.IconDataURL) == "" {
		result.addWarning("missing_icon", "Иконку приложения не удалось найти или декодировать.")
	}
}

func (result *Result) addWarning(code, message string) {
	for _, warning := range result.Warnings {
		if warning.Code == code {
			return
		}
	}
	result.Warnings = append(result.Warnings, Warning{Code: code, Message: message})
}

func packageType(name string) string {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(name))) {
	case ".dmg":
		return "dmg"
	case ".pkg", ".mpkg":
		return "pkg"
	case ".zip":
		return "zip"
	case ".app":
		return "app"
	default:
		return "other"
	}
}

func readZipFile(file *zip.File, max int64) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, errors.New("metadata file is too large")
	}
	return data, nil
}

func findZipIcon(files []*zip.File, prefix, iconName string) *zip.File {
	targets := iconCandidates(iconName)
	for _, file := range files {
		name := filepath.ToSlash(file.Name)
		if !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			continue
		}
		base := strings.ToLower(filepath.Base(name))
		for _, target := range targets {
			if base == strings.ToLower(target) {
				return file
			}
		}
	}
	return nil
}

func findIconOnDisk(resources, iconName string) string {
	targets := iconCandidates(iconName)
	for _, target := range targets {
		path := filepath.Join(resources, target)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	var found string
	_ = filepath.WalkDir(resources, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || found != "" || entry.IsDir() {
			return nil
		}
		base := strings.ToLower(filepath.Base(path))
		for _, target := range targets {
			if base == strings.ToLower(target) {
				found = path
			}
		}
		return nil
	})
	return found
}

func iconCandidates(name string) []string {
	name = strings.TrimSpace(filepath.Base(name))
	if name == "" {
		return nil
	}
	if filepath.Ext(name) != "" {
		return []string{name}
	}
	return []string{name + ".icns", name + ".png", name}
}

func iconDataURL(data []byte, name string) (string, int, int, error) {
	lower := strings.ToLower(name)
	var pngData []byte
	switch {
	case strings.HasSuffix(lower, ".png") || bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		pngData = data
	case strings.HasSuffix(lower, ".icns") || bytes.HasPrefix(data, []byte("icns")):
		pngData = extractPNG(data)
	default:
		pngData = extractPNG(data)
	}
	if len(pngData) == 0 {
		return "", 0, 0, errors.New("PNG representation not found")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(pngData))
	if err != nil {
		return "", 0, 0, err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngData), config.Width, config.Height, nil
}

func extractPNG(data []byte) []byte {
	signature := []byte("\x89PNG\r\n\x1a\n")
	for offset := bytes.Index(data, signature); offset >= 0; {
		pos := offset + len(signature)
		for pos+12 <= len(data) {
			length := int(uint32(data[pos])<<24 | uint32(data[pos+1])<<16 | uint32(data[pos+2])<<8 | uint32(data[pos+3]))
			if length < 0 || pos+12+length > len(data) {
				break
			}
			chunkType := string(data[pos+4 : pos+8])
			pos += 12 + length
			if chunkType == "IEND" {
				return append([]byte(nil), data[offset:pos]...)
			}
		}
		next := bytes.Index(data[offset+len(signature):], signature)
		if next < 0 {
			break
		}
		offset += len(signature) + next
	}
	return nil
}
