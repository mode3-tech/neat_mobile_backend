package appversion

type AppVersionInfoResponse struct {
	Android *android `json:"android,omitempty"`
	Ios     *ios     `json:"ios,omitempty"`
}

type android struct {
	MinBuild    int64  `json:"min_build"`
	LatestBuild int64  `json:"latest_build"`
	StoreURL    string `json:"store_url"`
}

type ios struct {
	MinBuild    int64  `json:"min_build,omitempty"`
	LatestBuild int64  `json:"latest_build,omitempty"`
	StoreURL    string `json:"store_url,omitempty"`
}
