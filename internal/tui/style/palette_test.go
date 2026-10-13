package style

import "testing"

func TestSetPaletteFillsStatusDefaults(t *testing.T) {
	SetPalette(Palette{Primary: "#ffffff"})
	p := CurrentPalette()
	d := DefaultPalette()
	if p.Error != d.Error || p.Warning != d.Warning || p.Success != d.Success {
		t.Fatalf("status defaults missing: %+v", p)
	}
	if p.Primary != "#ffffff" {
		t.Fatalf("primary overwritten: %q", p.Primary)
	}
	SetPalette(DefaultPalette())
}
