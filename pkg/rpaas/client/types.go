package client

type Flavor struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Plan struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     bool   `json:"default"`
}
