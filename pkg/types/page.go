package types

type ListPageResponse struct {
	Object  string         `json:"object"`
	Results []PageResponse `json:"results"`
}

type PageResponse struct {
	ID         string                            `json:"id"`
	Properties map[string]map[string]interface{} `json:"properties"`
}
