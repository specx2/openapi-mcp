package parser

import (
	"net/url"
	"path/filepath"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
)

func newDocument(spec []byte, specURL string) (libopenapi.Document, error) {
	if specURL == "" {
		return libopenapi.NewDocument(spec)
	}

	cfg := datamodel.NewDocumentConfiguration()

	u, err := url.Parse(specURL)
	if err != nil {
		return libopenapi.NewDocument(spec)
	}

	switch u.Scheme {
	case "", "file":
		cfg.BasePath = filepath.Dir(u.Path)
		cfg.SpecFilePath = filepath.Base(u.Path)
		cfg.AllowFileReferences = true
	case "http", "https":
		cfg.BaseURL = u
		cfg.AllowRemoteReferences = true
	default:
		return libopenapi.NewDocument(spec)
	}

	return libopenapi.NewDocumentWithConfiguration(spec, cfg)
}
