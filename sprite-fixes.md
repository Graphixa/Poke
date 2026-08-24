Yes, that makes sense, and I think it is the better abstraction.

The application should know about **rendering modes**, not terminal protocols.

So instead of modelling:

* KittyRenderer
* ITermRenderer
* SixelRenderer

you model:

* ImageRenderer
* HalfBlockRenderer
* BrailleRenderer
* ASCIIRenderer
* PlaceholderRenderer

Then `ImageRenderer` is responsible for whatever native image-capable path is available. If we use a library that already abstracts Kitty/iTerm/Sixel, great. If we later need a tiny bit of terminal-specific handling, it stays buried inside `ImageRenderer` and never leaks into the rest of the app.

That pattern is well proven: mature terminal image tools generally auto-detect a high-resolution protocol where available and otherwise fall back to Unicode block rendering. ([GitHub][1])

Here is the spec section I would add to `SPEC.md`.

# Sprite Rendering Engine

## 1. Purpose

Poke must render Pokémon sprites with the highest fidelity available in the current terminal while remaining portable across Windows, macOS, and Linux.

The rendering system must not expose individual terminal graphics protocols to the rest of the application.

Instead, sprite rendering is handled through a small set of logical renderer classes:

1. Native image
2. True-colour half-block
3. ANSI-256 half-block
4. Braille
5. ASCII
6. Text placeholder

The renderer subsystem is responsible for capability detection and selecting the best available mode automatically.

---

## 2. Design Principle

The rest of the application should not need to know whether a terminal supports Kitty graphics, iTerm image sequences, Sixel, or another native graphics mechanism.

The TUI should request:

> Render this sprite inside this rectangle.

The sprite subsystem decides how that should be achieved.

The intended abstraction is:

```text
Sprite
  │
  ▼
Renderer Resolver
  │
  ├── Image Renderer
  ├── TrueColour HalfBlock Renderer
  ├── ANSI-256 HalfBlock Renderer
  ├── Braille Renderer
  ├── ASCII Renderer
  └── Placeholder Renderer
```

Terminal-specific behaviour must remain encapsulated within the native image implementation or its supporting dependency.

---

## 3. Rendering Priority

The default automatic resolution order is:

```text
Native Image
      ↓ unavailable
TrueColour HalfBlock
      ↓ unavailable
ANSI-256 HalfBlock
      ↓ unavailable
Braille
      ↓ unavailable
ASCII
      ↓ unavailable
[ Pokémon Name ]
```

This order prioritises recognisability and colour fidelity.

---

## 4. Renderer Interface

All renderers should implement a common internal interface.

Conceptual Go API:

```go
type Renderer interface {
    Name() string

    Supported(caps Capabilities) bool

    Render(
        ctx context.Context,
        src image.Image,
        opts RenderOptions,
    ) (Frame, error)
}
```

Recommended options:

```go
type RenderOptions struct {
    Width       int
    Height      int
    PreserveAR  bool
    Animated    bool
    ColorMode   ColorMode
    Background  BackgroundMode
}
```

The renderer resolver should own renderer selection:

```go
type Resolver struct {
    renderers []Renderer
}

func (r *Resolver) Resolve(
    caps Capabilities,
    preference RenderMode,
) Renderer
```

No Bubble Tea component should select terminal protocols directly.

---

## 5. Native Image Renderer

### Purpose

Render the source sprite as a real image when the active terminal supports native image display.

### Architecture

The application exposes only:

```text
ImageRenderer
```

The implementation may internally use:

* a third-party terminal image abstraction
* an external protocol-detection library
* limited protocol-specific code if unavoidable

However, those details must not become part of Poke's public renderer architecture.

The contract is simply:

```text
Can this terminal display an image in this region?

Yes → render image
No  → return unsupported and allow fallback
```

### Recommendation

Prefer a maintained library that already performs:

* terminal capability detection
* native protocol selection
* image scaling
* image placement

rather than maintaining several native graphics protocols ourselves.

Projects in this space commonly expose exactly this type of automatic high-resolution-or-block-rendering behaviour. ([GitHub][1])

### Failure behaviour

If native image rendering:

* is unsupported
* cannot be reliably detected
* fails during rendering
* interferes with the TUI layout

the renderer must fail gracefully and allow the resolver to select HalfBlock.

Native image support must never be required for the TUI to function.

---

## 6. Half-Block Renderer

Half-block rendering is the primary non-native fallback and should be treated as a first-class feature.

### 6.1 Rendering technique

Use Unicode half-block characters:

```text
▀
```

Each terminal cell represents two vertically adjacent source samples:

```text
source pixel A
source pixel B

       ↓

foreground colour = A
background colour = B
character          = ▀
```

This produces an effective source sampling area of:

```text
terminal width × (terminal height × 2)
```

For example:

```text
30 columns × 15 rows
```

represents approximately:

```text
30 × 30 image samples
```

This is particularly suitable for the square Pokémon sprite panel.

Half-block rendering is commonly used as the general-purpose terminal fallback because it preserves colour significantly better than conventional character-ramp ASCII. ([GitHub][2])

---

## 7. TrueColour Half-Block Mode

When 24-bit terminal colour is available:

* preserve RGB colour directly
* use foreground RGB for the upper sample
* use background RGB for the lower sample

Conceptually:

```text
FG = RGB(topPixel)
BG = RGB(bottomPixel)
GLYPH = ▀
```

This should be the preferred fallback immediately after native image rendering.

For Pokémon sprites, this is expected to provide sufficient fidelity to identify most Pokémon even at modest dimensions.

---

## 8. ANSI-256 Half-Block Mode

When TrueColour is unavailable but 256-colour output is supported:

1. render internally in full RGB
2. resize and process normally
3. map each sample to the closest ANSI-256 colour
4. emit half-block cells using the mapped foreground/background colours

Colour quantisation should occur at the final rendering stage rather than earlier in the image pipeline.

Where practical, colour matching should use a perceptually weighted distance rather than simple raw RGB distance.

---

## 9. Image Preparation Pipeline

All text-based sprite renderers should share a common image preparation pipeline:

```text
Source Image
     ↓
Decode to RGBA
     ↓
Preserve transparency
     ↓
Determine target dimensions
     ↓
Preserve aspect ratio
     ↓
Resize using source-appropriate algorithm
     ↓
Renderer-specific conversion
```

This avoids each renderer implementing its own image preparation logic.

---

## 10. Sprite Source Selection

The engine should support at least two source categories.

### Pixel sprite

Best suited for terminal rendering.

Recommended behaviour:

* prefer standard game sprite
* use nearest-neighbour resizing
* preserve hard edges
* avoid unnecessary interpolation
* default choice for half-block rendering

### Artwork

Higher-resolution official artwork may optionally be supported.

Recommended behaviour:

* high-quality filtered downsampling
* suitable resampling such as Lanczos
* better suited for native image rendering

Default renderer behaviour should favour pixel sprites unless configuration specifies otherwise.

---

## 11. Transparency

Sprite transparency must be preserved.

Transparent areas must not be flattened onto a white background before conversion.

Recommended handling:

```text
alpha < threshold
    → transparent terminal cell/sample
```

Transparent image regions should map to:

* spaces
* the current terminal background
* or an unset foreground/background where appropriate

A configurable or implementation-level alpha threshold may be used to prevent anti-aliased sprite edges from producing visual noise.

---

## 12. Aspect Ratio

Terminal cells are not square.

The renderer must compensate for cell geometry.

For half-block rendering:

```text
targetPixelWidth  = panelColumns
targetPixelHeight = panelRows × 2
```

The source image is then resized to fit inside this logical pixel rectangle while preserving its aspect ratio.

The finished sprite should be centred horizontally and vertically within the sprite card.

---

## 13. Resampling

Use different resizing behaviour according to sprite source.

### Pixel-art sources

Use:

```text
Nearest Neighbour
```

This preserves crisp sprite edges.

### Artwork sources

Use a high-quality downsampling filter such as:

```text
Lanczos
```

The implementation should avoid using smoothing filters on deliberately pixelated sprites.

---

## 14. Dithering

Dithering should not be enabled by default.

Pokémon sprites tend to contain:

* strong outlines
* large flat colour regions
* distinct colour boundaries

Dithering can introduce undesirable noise into those areas.

Initial implementation:

```yaml
render:
  dithering: none
```

Dithering may become an optional setting later for artwork or reduced-colour terminals.

---

## 15. Braille Renderer

Braille is the preferred high-resolution monochrome fallback.

Each Braille character represents a:

```text
2 × 4
```

dot grid.

This allows substantially greater spatial resolution than traditional ASCII characters.

Braille rendering should focus on:

* silhouette
* high-contrast features
* alpha/transparency mask
* edge preservation

The Braille renderer should be used when colour block rendering is unavailable or explicitly requested.

---

## 16. ASCII Renderer

Classic ASCII rendering is a compatibility fallback rather than the primary sprite-rendering strategy.

Typical luminance characters may include:

```text
 .:-=+*#%@
```

ASCII rendering should:

* convert image samples to luminance
* preserve aspect ratio
* honour transparency
* prioritise silhouette recognition over photographic accuracy

The renderer may later gain edge-aware processing, but this is not required for v1.

---

## 17. Placeholder Renderer

The final fallback must always work.

Example:

```text
[ Charmander ]
```

The placeholder should:

* be centred inside the sprite panel
* use normal TUI styling
* never produce a rendering error

This ensures missing images or rendering failures never break browse mode.

---

## 18. Capability Detection

Capabilities should be detected once during application initialisation and stored centrally.

Example:

```go
type Capabilities struct {
    NativeImage bool
    TrueColor   bool
    ANSI256     bool
    Unicode     bool
    Braille     bool
}
```

The TUI should not repeatedly probe the terminal during rendering.

Detection should be conservative:

* uncertain native image support should result in fallback
* uncertain colour support should use the safer mode
* explicit user configuration may override automatic detection

---

## 19. Configuration

Suggested YAML configuration:

```yaml
render:
  sprite_mode: auto
  sprite_source: pixel
  color: auto
  dithering: none
  animate: auto
```

Supported `sprite_mode` values:

```text
auto
image
halfblock
braille
ascii
off
```

`auto` uses the complete fallback chain.

`image` should still fall back safely if native image rendering fails unless a strict mode is introduced later.

---

## 20. Caching

Converted sprite output should be cached.

The engine should not repeatedly:

* download
* decode
* resize
* colour-quantise
* convert

the same sprite while the user navigates.

Suggested cache identity:

```text
pokemon ID
sprite source
renderer mode
render width
render height
colour mode
render algorithm version
```

Example:

```text
004:pixel:halfblock:32x16:truecolor:v1
```

### Size bucketing

Avoid creating a cache entry for every possible terminal size.

Render sizes should be quantised into buckets such as:

```text
16
20
24
28
32
36
40
```

A requested width should use the nearest suitable bucket.

This prevents terminal resizing from producing excessive cache artefacts.

---

## 21. TUI Integration

Text-based renderers:

* HalfBlock
* Braille
* ASCII
* Placeholder

return ordinary terminal text and can be composed normally through Bubble Tea and Lip Gloss.

The native ImageRenderer may require a different integration strategy depending on the chosen library.

The renderer subsystem must hide this distinction from higher-level browse components as much as possible.

Conceptually:

```go
type Frame struct {
    Text  string
    Image *ImageFrame
}
```

The sprite component receives a `Frame` and renders it according to type.

---

## 22. Responsive Behaviour

The sprite renderer only operates while the sprite panel itself is visible.

If the viewport falls below the configured minimum dimensions:

```text
hide sprite panel
```

The engine should not attempt to render a smaller degraded sprite.

The freed vertical space returns to the Pokémon result list.

This rule applies regardless of rendering mode.

---

## 23. Animation

Animation is optional and should not be required for v1.

If supported later:

### Native image rendering

Prefer native animation support where available through the selected image abstraction.

### Text rendering

Animated source frames may be pre-rendered and cycled using Bubble Tea timer messages.

Animation should be disabled automatically if it causes excessive redraw cost or terminal instability.

Suggested configuration:

```yaml
render:
  animate: auto
```

Possible values later:

```text
auto
on
off
```

---

## 24. Implementation Phases

### Phase 1 — Core engine

Build:

* shared image preparation pipeline
* renderer interface
* capability model
* resolver
* placeholder renderer

### Phase 2 — HalfBlock

Build:

* TrueColour half-block renderer
* transparency handling
* resizing
* aspect ratio correction
* centring
* render caching

This should be the first visually complete renderer.

### Phase 3 — Reduced-colour support

Build:

* ANSI-256 detection
* colour quantisation
* ANSI-256 half-block output

### Phase 4 — Monochrome fallbacks

Build:

* Braille renderer
* ASCII renderer

### Phase 5 — Native image support

Integrate a native image rendering abstraction.

Requirements:

* auto-detection
* image placement
* sizing
* safe failure
* no native protocol types exposed outside the image renderer package

### Phase 6 — Optimisation

Add:

* size-bucket cache
* decode cache
* optional pre-rendering
* performance benchmarks

### Phase 7 — Optional animation

Only after static rendering is stable.

---

## 25. Recommended Package Structure

```text
internal/render/sprite/
    renderer.go
    resolver.go
    capabilities.go
    options.go
    frame.go
    imageprep.go
    resize.go
    cache.go

    image/
        renderer.go

    halfblock/
        renderer.go
        color.go

    braille/
        renderer.go

    ascii/
        renderer.go

    placeholder/
        renderer.go
```

There should not be top-level packages such as:

```text
kitty/
iterm/
sixel/
```

unless a future native-image implementation requires them internally.

Even then, they should remain private implementation details beneath:

```text
image/
```

---

## 26. Acceptance Criteria

The rendering subsystem is considered v1-ready when:

* Pokémon sprites render recognisably in TrueColour half-block mode
* transparent backgrounds remain visually clean
* sprite aspect ratio is preserved
* resize operations do not distort sprites
* ANSI-256 fallback works
* Braille fallback works
* ASCII fallback works
* placeholder output always works
* native image failure automatically falls back
* renderer selection is automatic
* user configuration can override renderer mode
* resize events do not create excessive cached variants
* sprite rendering never blocks or breaks normal TUI navigation

One architectural distinction I would keep very firm: **capability detection chooses a rendering class, not a terminal brand**. That will keep this subsystem maintainable. The fact that a native image library may internally determine “this is Kitty-compatible” is irrelevant to the rest of Poke.

[1]: https://github.com/sindresorhus/terminal-image?utm_source=chatgpt.com "GitHub - sindresorhus/terminal-image: Display images in the terminal · GitHub"
[2]: https://github.com/hzeller/timg?utm_source=chatgpt.com "GitHub - hzeller/timg: A terminal image and video viewer. · GitHub"
