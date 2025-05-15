package blossom

type InformationDocument struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	Banner          string `json:"banner"`
	Icon            string `json:"icon"`
	PubKey          string `json:"pubkey"`
	Contact         string `json:"contact"`
	SupportedBUDs   []int  `json:"supported_buds"`
	Software        string `json:"software"`
	UploadingPolicy string `json:"uploading_policy"`
	Version         string `json:"version"`
}
