package jav

import (
	"javboss/internal/jav/metadata"
)

// Aliases keep the query API compatible with existing metadata consumers.
type Provider = metadata.Provider
type JavInfo = metadata.JavInfo
type ActressInfo = metadata.ActressInfo
type SampleImage = metadata.SampleImage
type JavBusGenreCategory = metadata.GenreCategory

const (
	ProviderUnknown      = metadata.ProviderUnknown
	ProviderJavBus       = metadata.ProviderJavBus
	ProviderJavDatabase  = metadata.ProviderJavDatabase
	ProviderUser         = metadata.ProviderUser
	ProviderJavDB        = metadata.ProviderJavDB
	ProviderAvmoo        = metadata.ProviderAvmoo
	ProviderThePornDB    = metadata.ProviderThePornDB
	ProviderJavModel     = metadata.ProviderJavModel
	ProviderAvsox        = metadata.ProviderAvsox
	ProviderJavMenu      = metadata.ProviderJavMenu
	ProviderMinnanoAV    = metadata.ProviderMinnanoAV
	ProviderManualScrape = metadata.ProviderManualScrape
	ProviderJavDBAPI     = metadata.ProviderJavDBAPI
	ProviderAVWiki       = metadata.ProviderAVWiki
)

func ParseProvider(value int) Provider { return metadata.ParseProvider(value) }
