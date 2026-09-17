package account

type AdminVersion struct {
	ID            int64  `json:"id"`
	AppID         int64  `json:"app_id"`
	Version       string `json:"version"`
	ReleaseDate   string `json:"release_date,omitempty"`
	Changelog     string `json:"changelog,omitempty"`
	IsRecommended bool   `json:"is_recommended"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type AdminArtifact struct {
	ID                 int64  `json:"id"`
	AppVersionID       int64  `json:"app_version_id"`
	FileName           string `json:"file_name"`
	PackageType        string `json:"package_type"`
	SourceType         string `json:"source_type"`
	StoragePath        string `json:"storage_path,omitempty"`
	PrimaryDownloadURL string `json:"primary_download_url,omitempty"`
	TorrentURL         string `json:"torrent_url,omitempty"`
	MagnetURL          string `json:"magnet_url,omitempty"`
	SizeBytes          int64  `json:"size_bytes"`
	SHA256             string `json:"sha256,omitempty"`
	MinOS              string `json:"min_os"`
	MaxSupportedOS     string `json:"max_supported_os,omitempty"`
	MaxTestedOS        string `json:"max_tested_os,omitempty"`
	HardBlockAboveMax  bool   `json:"hard_block_above_max"`
	ArchI386           bool   `json:"arch_i386"`
	ArchX8664          bool   `json:"arch_x86_64"`
	Supports32Bit      bool   `json:"supports_32bit"`
	Supports64Bit      bool   `json:"supports_64bit"`
	RequiresRosetta    bool   `json:"requires_rosetta"`
	RequiresJava       bool   `json:"requires_java"`
	InstallNotes       string `json:"install_notes,omitempty"`
	ModerationStatus   string `json:"moderation_status"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

type AdminMirror struct {
	ID         int64  `json:"id"`
	ArtifactID int64  `json:"artifact_id"`
	MirrorType string `json:"mirror_type"`
	URL        string `json:"url"`
	Priority   int    `json:"priority"`
	IsActive   bool   `json:"is_active"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type AdminIcon struct {
	ID           int64  `json:"id"`
	AppID        int64  `json:"app_id"`
	AppVersionID int64  `json:"app_version_id,omitempty"`
	ImageURL     string `json:"image_url"`
	MinOS        string `json:"min_os"`
	MaxOS        string `json:"max_os"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type AdminScreenshot struct {
	ID           int64  `json:"id"`
	AppID        int64  `json:"app_id"`
	AppVersionID int64  `json:"app_version_id,omitempty"`
	ImageURL     string `json:"image_url"`
	MinOS        string `json:"min_os"`
	MaxOS        string `json:"max_os"`
	Caption      string `json:"caption,omitempty"`
	SortOrder    int    `json:"sort_order"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}
