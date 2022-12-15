package models

import "github.com/Kaese72/adapter-attendant/database/intermediaries"

type Image struct {
	Name string `json:"name"`
	Tag  string `json:"Tag"`
}

type Adapter struct {
	Name    string            `json:"name"`
	Image   *Image            `json:"image"`
	Config  map[string]string `json:"config"`
	Address string            `json:"address"` // readonly
}

func (adapter Adapter) Intermediary() intermediaries.Adapter {
	var image *intermediaries.Image = nil
	if adapter.Image != nil {
		image = &intermediaries.Image{
			Name: adapter.Image.Name,
			Tag:  adapter.Image.Tag,
		}
	}
	return intermediaries.Adapter{
		Name:    adapter.Name,
		Image:   image,
		Config:  adapter.Config,
		Address: adapter.Address,
	}
}

func AdapterFromIntermediary(intermediary intermediaries.Adapter) Adapter {
	var image *Image = nil
	if intermediary.Image != nil {
		image = &Image{
			Name: intermediary.Image.Name,
			Tag:  intermediary.Image.Tag,
		}
	}
	return Adapter{
		Name:    intermediary.Name,
		Image:   image,
		Config:  intermediary.Config,
		Address: intermediary.Address,
	}
}
