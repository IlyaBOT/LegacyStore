package compatibility

import (
	"strings"

	"legacystore/backend/internal/architecture"
)

type Target struct {
	OSVersion string
	Arch      string
	// OSSeries treats OSVersion as a major.minor catalog family (for example 10.6.x).
	// Native clients should leave this false and send the exact host OS version.
	OSSeries bool
}

type Artifact struct {
	MinOS             string
	MaxSupportedOS    string
	MaxTestedOS       string
	HardBlockAboveMax bool
	Architectures     []string
	RequiresRosetta   bool
	RequiresJava      bool
}

type Reason struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Result struct {
	Status  string   `json:"status"`
	Level   string   `json:"level,omitempty"`
	Label   string   `json:"label,omitempty"`
	Reasons []Reason `json:"reasons,omitempty"`
}

func Evaluate(target Target, artifact Artifact) Result {
	if target.OSVersion == "" {
		target.OSVersion = "10.9.5"
	}
	if target.Arch == "" {
		target.Arch = "x86_64"
	}

	osVersion, err := ParseVersion(target.OSVersion)
	if err != nil {
		return blocked(reason("unknown_compatibility", "Не удалось определить версию системы."))
	}

	var reasons []Reason

	if artifact.MinOS != "" {
		minOS, err := ParseVersion(artifact.MinOS)
		if err != nil {
			reasons = append(reasons, reason("unknown_compatibility", "Минимальная версия системы указана в неизвестном формате."))
		} else if compareTargetOS(osVersion, minOS, target.OSSeries) < 0 {
			reasons = append(reasons, reason("os_too_old", "Версия системы "+target.OSVersion+" ниже минимальной требуемой продуктом "+artifact.MinOS+"."))
		}
	}

	if artifact.MaxSupportedOS != "" {
		maxOS, err := ParseVersion(artifact.MaxSupportedOS)
		if err != nil {
			reasons = append(reasons, reason("unknown_compatibility", "Максимальная версия системы указана в неизвестном формате."))
		} else if compareTargetOS(osVersion, maxOS, target.OSSeries) > 0 && artifact.HardBlockAboveMax {
			reasons = append(reasons, reason("os_too_new", "Версия системы "+target.OSVersion+" выше максимальной поддерживаемой продуктом "+artifact.MaxSupportedOS+"."))
		}
	}

	if len(artifact.Architectures) == 0 {
		reasons = append(reasons, reason("unknown_compatibility", "Для сборки не указана архитектура процессора."))
	} else if !architecture.Compatible(target.Arch, artifact.Architectures, osVersion.Supports32BitApps()) {
		required := strings.Join(architecture.HumanLabels(artifact.Architectures), ", ")
		if required == "" {
			required = strings.Join(artifact.Architectures, ", ")
		}
		reasons = append(reasons, reason(
			"arch_mismatch",
			"Архитектура вашего Mac "+target.Arch+" не поддерживается этой сборкой. Требуется: "+required+".",
		))
	}

	if artifact.RequiresRosetta {
		reasons = append(reasons, reason("requires_rosetta", "Продукт требует Rosetta, что не поддерживается в этой версии LegacyStore."))
	}

	if hasBlockingReason(reasons) {
		return Result{
			Status:  "blocked",
			Level:   "blocked",
			Label:   "Не совместимо",
			Reasons: reasons,
		}
	}

	if artifact.RequiresJava {
		reasons = append(reasons, reason("requires_java", "Для продукта может потребоваться Java."))
	}

	if artifact.MaxTestedOS != "" {
		maxTested, err := ParseVersion(artifact.MaxTestedOS)
		if err == nil && compareTargetOS(osVersion, maxTested, target.OSSeries) > 0 {
			reasons = append(reasons, reason("untested_newer_os", "Версия системы "+target.OSVersion+" новее максимальной протестированной версии "+artifact.MaxTestedOS+"."))
			return Result{
				Status:  "untested",
				Level:   "untested",
				Label:   "Не проверено",
				Reasons: reasons,
			}
		}
	}

	if len(reasons) > 0 {
		return Result{
			Status:  "probably_compatible",
			Level:   "probably_compatible",
			Label:   "Вероятно совместимо",
			Reasons: reasons,
		}
	}

	return Result{Status: "compatible", Level: "recommended", Label: "Совместимо"}
}

func compareTargetOS(target, boundary Version, series bool) int {
	if !series {
		return target.Compare(boundary)
	}
	if target.Major != boundary.Major {
		return compareInt(target.Major, boundary.Major)
	}
	return compareInt(target.Minor, boundary.Minor)
}

func blocked(r Reason) Result {
	return Result{
		Status:  "blocked",
		Level:   "blocked",
		Label:   "Не совместимо",
		Reasons: []Reason{r},
	}
}

func reason(code, message string) Reason {
	return Reason{Code: code, Message: message}
}

func hasBlockingReason(reasons []Reason) bool {
	for _, r := range reasons {
		switch r.Code {
		case "os_too_old", "os_too_new", "arch_mismatch", "requires_rosetta", "unknown_compatibility":
			return true
		}
	}
	return false
}
