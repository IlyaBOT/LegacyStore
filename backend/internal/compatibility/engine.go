package compatibility

type Target struct {
	OSVersion string
	Arch      string
}

type Artifact struct {
	MinOS             string
	MaxSupportedOS    string
	MaxTestedOS       string
	HardBlockAboveMax bool
	ArchI386          bool
	ArchX8664         bool
	Supports32Bit     bool
	Supports64Bit     bool
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
		} else if osVersion.Compare(minOS) < 0 {
			reasons = append(reasons, reason("os_too_old", "Версия системы "+target.OSVersion+" ниже минимальной требуемой продуктом "+artifact.MinOS+"."))
		}
	}

	if artifact.MaxSupportedOS != "" {
		maxOS, err := ParseVersion(artifact.MaxSupportedOS)
		if err != nil {
			reasons = append(reasons, reason("unknown_compatibility", "Максимальная версия системы указана в неизвестном формате."))
		} else if osVersion.Compare(maxOS) > 0 && artifact.HardBlockAboveMax {
			reasons = append(reasons, reason("os_too_new", "Версия системы "+target.OSVersion+" выше максимальной поддерживаемой продуктом "+artifact.MaxSupportedOS+"."))
		}
	}

	switch target.Arch {
	case "i386":
		if !artifact.ArchI386 {
			reasons = append(reasons, reason("arch_mismatch", "Архитектура вашего Mac i386 не поддерживается. Требуется: x86_64."))
		}
		if !artifact.Supports32Bit {
			reasons = append(reasons, reason("requires_64bit", "Продукт требует 64-битную систему."))
		}
	case "x86_64":
		if !artifact.ArchX8664 {
			reasons = append(reasons, reason("arch_mismatch", "Архитектура вашего Mac x86_64 не поддерживается. Требуется: i386."))
		}
	default:
		reasons = append(reasons, reason("arch_mismatch", "Архитектура вашего Mac не поддерживается."))
	}

	if !osVersion.Supports32BitApps() && !artifact.Supports64Bit {
		reasons = append(reasons, reason("requires_32bit", "Продукт является 32-битным, а macOS 10.15 поддерживает только 64-битные приложения."))
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
		if err == nil && osVersion.Compare(maxTested) > 0 {
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
		case "os_too_old", "os_too_new", "arch_mismatch", "bitness_mismatch", "requires_32bit", "requires_64bit", "requires_rosetta", "unknown_compatibility":
			return true
		}
	}
	return false
}
