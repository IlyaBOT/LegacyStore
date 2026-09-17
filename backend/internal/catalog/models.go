package catalog

import "legacystore/backend/internal/compatibility"

type Category struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order,omitempty"`
}

type AppSummary struct {
	Slug               string               `json:"slug"`
	Name               string               `json:"name"`
	Category           string               `json:"category"`
	Summary            string               `json:"summary"`
	Icon               string               `json:"icon,omitempty"`
	HeroImage          string               `json:"hero_image,omitempty"`
	Rating             float64              `json:"rating"`
	RatingCount        int                  `json:"rating_count"`
	Downloads          int64                `json:"downloads"`
	RecommendedVersion string               `json:"recommended_version,omitempty"`
	ArchBadges         []string             `json:"arch_badges"`
	Compatibility      compatibility.Result `json:"compatibility"`
}

type HomeSlide struct {
	Metric string     `json:"metric"`
	App    AppSummary `json:"app"`
}

type HomeFeed struct {
	Popular      []AppSummary `json:"popular"`
	TopDownloads []AppSummary `json:"top_downloads"`
	NewReleases  []AppSummary `json:"new_releases"`
	Slides       []HomeSlide   `json:"slides"`
}

type AppDetail struct {
	Slug                string               `json:"slug"`
	Name                string               `json:"name"`
	BundleID            string               `json:"bundle_id,omitempty"`
	DeveloperName       string               `json:"developer_name"`
	Summary             string               `json:"summary"`
	Description         string               `json:"description,omitempty"`
	Category            string               `json:"category"`
	WebsiteURL          string               `json:"website_url,omitempty"`
	LicenseType         string               `json:"license_type,omitempty"`
	Icon                string               `json:"icon,omitempty"`
	Screenshots         []Screenshot         `json:"screenshots,omitempty"`
	Rating              float64              `json:"rating"`
	RatingCount         int                  `json:"rating_count"`
	RecommendedArtifact *ArtifactResponse    `json:"recommended_artifact,omitempty"`
	Versions            []VersionResponse    `json:"versions"`
	Compatibility       compatibility.Result `json:"compatibility"`
}

type VersionResponse struct {
	Version       string             `json:"version"`
	ReleaseDate   string             `json:"release_date,omitempty"`
	Changelog     string             `json:"changelog,omitempty"`
	IsRecommended bool               `json:"is_recommended"`
	Artifacts     []ArtifactResponse `json:"artifacts"`
}

type ArtifactResponse struct {
	ID                  int64    `json:"id"`
	Version             string   `json:"version,omitempty"`
	FileName            string   `json:"file_name"`
	PackageType         string   `json:"package_type"`
	SourceType          string   `json:"source_type"`
	DownloadURL         string   `json:"download_url,omitempty"`
	ExternalPageURL     string   `json:"external_page_url,omitempty"`
	TorrentURL          string   `json:"torrent_url,omitempty"`
	MagnetURL           string   `json:"magnet_url,omitempty"`
	SizeBytes           int64    `json:"size_bytes"`
	SHA256              string   `json:"sha256"`
	MinOS               string   `json:"min_os"`
	MaxSupportedOS      string   `json:"max_supported_os,omitempty"`
	MaxTestedOS         string   `json:"max_tested_os,omitempty"`
	HardBlockAboveMax   bool     `json:"hard_block_above_max,omitempty"`
	Archs               []string `json:"archs"`
	Supports32Bit       bool     `json:"supports_32bit"`
	Supports64Bit       bool     `json:"supports_64bit"`
	RequiresJava        bool     `json:"requires_java,omitempty"`
	InstallNotes        string   `json:"install_notes,omitempty"`
	CompatibilityLabel  string   `json:"compatibility_label,omitempty"`
	CompatibilityStatus string   `json:"compatibility_status,omitempty"`
}

type Screenshot struct {
	ImageURL  string `json:"image_url"`
	Caption   string `json:"caption,omitempty"`
	SortOrder int    `json:"sort_order"`
}

type DownloadMetadata struct {
	ArtifactResponse
	Mirrors []Mirror `json:"mirrors,omitempty"`
}

type Mirror struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Priority int    `json:"priority"`
}

type Review struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id,omitempty"`
	Author    string `json:"author,omitempty"`
	Rating    int    `json:"rating"`
	Title     string `json:"title,omitempty"`
	Body      string `json:"body,omitempty"`
	Likes     int    `json:"likes"`
	Replies   int    `json:"replies"`
	CreatedAt string `json:"created_at,omitempty"`
}

type Filters struct {
	Page           int
	Limit          int
	Category       string
	Query          string
	Sort           string
	Target         compatibility.Target
	CompatibleOnly bool
}
