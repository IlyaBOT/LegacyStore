package account

import "encoding/json"

type StagedUpload struct {
	UID                   string          `json:"uid"`
	AppID                 int64           `json:"app_id"`
	SubmittedBy           int64           `json:"submitted_by"`
	OriginalFilename      string          `json:"original_filename"`
	StoragePath           string          `json:"-"`
	SHA256                string          `json:"sha256"`
	SizeBytes             int64           `json:"size_bytes"`
	PackageType           string          `json:"package_type"`
	DetectedName          string          `json:"detected_name,omitempty"`
	DetectedBundleID      string          `json:"detected_bundle_id,omitempty"`
	DetectedVersion       string          `json:"detected_version,omitempty"`
	DetectedCategorySlug  string          `json:"detected_category_slug,omitempty"`
	DetectedMinOS         string          `json:"detected_min_os,omitempty"`
	DetectedArchitectures []string        `json:"detected_architectures,omitempty"`
	Warnings              json.RawMessage `json:"warnings,omitempty"`
	Status                string          `json:"status"`
	ExpiresAt             string          `json:"expires_at"`
	CommittedAt           string          `json:"committed_at,omitempty"`
	CreatedAt             string          `json:"created_at"`
	UpdatedAt             string          `json:"updated_at"`
}

type StageUploadInput struct {
	AppID                 int64
	SubmittedBy           int64
	OriginalFilename      string
	StoragePath           string
	SHA256                string
	SizeBytes             int64
	PackageType           string
	DetectedName          string
	DetectedBundleID      string
	DetectedVersion       string
	DetectedCategorySlug  string
	DetectedMinOS         string
	DetectedArchitectures []string
	Warnings              json.RawMessage
}

type ReleaseSubmission struct {
	Version           string   `json:"version"`
	ReleaseDate       string   `json:"release_date,omitempty"`
	Changelog         string   `json:"changelog,omitempty"`
	IsRecommended     bool     `json:"is_recommended"`
	MinOS             string   `json:"min_os"`
	MaxSupportedOS    string   `json:"max_supported_os,omitempty"`
	MaxTestedOS       string   `json:"max_tested_os,omitempty"`
	HardBlockAboveMax bool     `json:"hard_block_above_max"`
	Architectures     []string `json:"architectures"`
	RequiresRosetta   bool     `json:"requires_rosetta"`
	RequiresJava      bool     `json:"requires_java"`
	InstallNotes      string   `json:"install_notes,omitempty"`
}

type ReleaseSubmissionResult struct {
	AppID            int64         `json:"app_id"`
	VersionID        int64         `json:"version_id"`
	Version          string        `json:"version"`
	Artifact         AdminArtifact `json:"artifact"`
	ModerationStatus string        `json:"moderation_status"`
}
