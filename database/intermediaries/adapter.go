package intermediaries

type Image struct {
	Name string
	Tag  string
}

type Adapter struct {
	Name    string
	Image   *Image
	Config  map[string]string
	Address string
}
