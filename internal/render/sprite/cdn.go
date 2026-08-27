package sprite

import "fmt"

// PokeAPISpritesCDN is the stable raw GitHub host for PokeAPI sprite PNGs.
const PokeAPISpritesCDN = "https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon"

// CDNPixelURL returns the front_default pixel sprite URL for a national dex ID.
func CDNPixelURL(id int) string {
	if id < 1 {
		return ""
	}
	return fmt.Sprintf("%s/%d.png", PokeAPISpritesCDN, id)
}

// CDNArtworkURL returns the official-artwork URL for a national dex ID.
func CDNArtworkURL(id int) string {
	if id < 1 {
		return ""
	}
	return fmt.Sprintf("%s/other/official-artwork/%d.png", PokeAPISpritesCDN, id)
}

// NamedAssetKey is a stable content identity for named on-disk sprites so
// Resolve can skip SHA-256 on named hits (path already identifies the asset).
func NamedAssetKey(id int, slug, kind string) string {
	if slug == "" {
		slug = "_"
	}
	if kind == "" {
		kind = "pixel"
	}
	return fmt.Sprintf("named-%d-%s-%s", id, slug, kind)
}
