package certassets

// PrintAssetDTO opisuje wgrany nadruk. Bajty pliku nigdy tędy nie wychodzą -
// od tego jest osobna trasa zwracająca obraz.
type PrintAssetDTO struct {
	Kind         string `json:"kind"`
	FileName     string `json:"fileName"`
	ContentType  string `json:"contentType"`
	FileSize     int64  `json:"fileSize"`
	PrintWidthMm int    `json:"printWidthMm"`
	UploadedAt   string `json:"uploadedAt"`
}

type ListPrintAssetsResponse struct {
	Data []PrintAssetDTO `json:"data"`
}

type PrintAssetResponse struct {
	Data PrintAssetDTO `json:"data"`
}
