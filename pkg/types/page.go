package types

type ListPageResponse struct {
	Object  string         `json:"object"`
	Results []PageResponse `json:"results"`
}

type PageResponse struct {
	ID string `json:"id"`
}
