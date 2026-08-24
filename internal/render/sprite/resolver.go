package sprite

// ModeResolver selects a Renderer from capabilities and user preference.
type ModeResolver struct {
	Renderers []Renderer
}

// NewModeResolver builds the default fallback chain:
//
//	image → chafa mosaic → placeholder
func NewModeResolver(renderers ...Renderer) *ModeResolver {
	return &ModeResolver{Renderers: renderers}
}

// Resolve picks the best supported renderer for preference.
// ModeAuto walks the full chain. An explicit mode is returned when present in
// the chain so the engine can attempt it even if capability detection is
// uncertain; render failure still falls forward. ModeOff forces placeholder.
func (r *ModeResolver) Resolve(caps Capabilities, preference RenderMode) Renderer {
	if r == nil || len(r.Renderers) == 0 {
		return nil
	}
	pref := preference
	if pref == "" {
		pref = ModeAuto
	}

	switch pref {
	case ModeOff, ModePlaceholder:
		return r.findByName("placeholder")
	case ModeImage, ModeChafa:
		if rend := r.findPreferred(pref); rend != nil {
			return rend
		}
		// Preferred mode not registered → continue auto chain after that mode.
		return r.resolveAuto(caps, skipBefore(pref))
	default: // auto
		return r.resolveAuto(caps, "")
	}
}

func (r *ModeResolver) resolveAuto(caps Capabilities, skipUntil string) Renderer {
	skipping := skipUntil != ""
	for _, rend := range r.Renderers {
		if skipping {
			if rend.Name() == skipUntil {
				skipping = false
			}
			continue
		}
		if rend.Supported(caps) {
			return rend
		}
	}
	return r.findByName("placeholder")
}

func (r *ModeResolver) findPreferred(mode RenderMode) Renderer {
	name := string(mode)
	for _, rend := range r.Renderers {
		if rend.Name() == name {
			return rend
		}
	}
	return nil
}

func (r *ModeResolver) findByName(name string) Renderer {
	for _, rend := range r.Renderers {
		if rend.Name() == name {
			return rend
		}
	}
	if len(r.Renderers) > 0 {
		return r.Renderers[len(r.Renderers)-1]
	}
	return nil
}

func skipBefore(mode RenderMode) string {
	// After a failed preferred mode, resume auto at the next class.
	switch mode {
	case ModeImage:
		return "image"
	case ModeChafa:
		return "chafa"
	default:
		return ""
	}
}
