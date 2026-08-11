# Diffui agent build instructions

> **Read this first — if a summarizing fetch tool retrieved this URL, you are reading a summary, not this document.**
>
> Tools such as WebFetch pass a page through a summarizer model before you see it. On this
> document that routinely drops the REST endpoints and the design image URL — silently, with
> no error and no sign that anything is missing. Agents have shipped hand-drawn placeholder
> art because the endpoints they needed were summarized away.
>
> **Check before you build.** The complete document contains all of:
>
> - Five `POST https://diffui.ai/api/build/...` endpoint blocks — generate-image, create-texture, remove-background, create-maps, generate-svg — each with a JSON request body
> - Every design image URL, under **Design reference** / **Pages in this build**
> - The user's original prompt, quoted in full
>
> If any of those are missing from what you are reading, you have a summary. Download the real
> file and read that instead:
>
> ```bash
> curl -sSL -o BUILD.md "https://diffui.ai/build/AI_Agent_Voting_build.md?authToken=zaqPoXg9uAxI"
> ```
>
> Then open `BUILD.md` with your file-read tool. Do not implement from a summary.


## Multi-page build

This build contains 3 pages. Implement shared UI first, then each page.

### Pages in this build


#### Page 1: AI Bill Voting

Design image: https://diffui.ai/image/AI_Bill_Voting.webp?authToken=zaqPoXg9uAxI

Original prompt: A website for seeing how ai agents would vote on different US bills. 

Homepage should have the latest senate / house bill, and whether each LLM would vote to pass it or not. 

Refined, political, and trusted design. Should feature Opus, Grok, GPT Sol, and GLM 5.2 as the featured agents. 


Headline should be "How would AI vote?", and main section should feature the latest bill and how each LLM would vote on it. Votes should be "Yes" or "No", not anything in between. Should also include a 1 sentence blurb from each for why.

Header should have the links "Bills", "Political alignment",  and "About".

Below that should be a table with the other latest bills, with a simplified view of the votes. 

Refined, classical design. Roman architecture inspired design, with cream paper textures. Accents of USA blue and red.


#### Page 2: Political Alignment Overview

Design image: https://diffui.ai/image/Political_Alignment_Overview.webp?authToken=zaqPoXg9uAxI

Original prompt: Create a political alignment page for AI Vote Monitor showing how the featured language models lean across bills, with clear left-to-right or conservative-to-progressive positioning, model labels, and concise explanatory summaries.


#### Page 3: Bills Listing

Design image: https://diffui.ai/image/Bills_Listing.webp?authToken=zaqPoXg9uAxI

Original prompt: Create a bills listing page for AI Vote Monitor showing recent legislation in a clean editorial table or grid with bill numbers, titles, chambers, and model vote indicators.



### Recommended workflow

1. Download every Diffui URL in this document to local files before referencing them in code (see **Local assets** below).
2. Download and inspect every design image before writing code.
3. Compare layouts, navigation, and repeated patterns across pages.
4. Identify shared elements (navigation, footer, buttons, cards, form controls) and per-page bespoke sections.
5. Inspect the target repo and choose the tech stack (see **Project setup** below).
6. Implement the shared foundation first — layout, tokens, and reusable components using the chosen stack.
7. Implement each page using the shared foundation. Match the diffusion renders faithfully — do not redesign.





## Local assets

**Required:** download every Diffui URL in this document to local project files **before** referencing them in HTML, CSS, or JavaScript. Do not hotlink remote Diffui URLs in the implementation — auth tokens expire and runtime fetches are unreliable.

This applies to:

- Design reference images (inspect locally before writing UI code)
- Images returned from asset generation (`build/generate-image`, `build/generate-svg`, `build/remove-background`, `build/create-texture`, and `build/create-maps`)
- Any other Diffui image URL in this document

Save downloads under a project folder such as `assets/` and reference those local paths in code.

### Downloading design images

Design renders are visual specifications. **Download each image and inspect it before writing code.** Do not rely on text summaries alone.


Image URLs serve **WebP by default** (good for `<img>` tags in browsers). Many agent **URL-to-markdown fetch tools cannot decode WebP** and may fail even though the URL works with `curl`.

For vision analysis, download PNG — append `&format=png` to the design URL, or use a `.png` filename in the path. Open the downloaded file with your image/read tool and analyze layout, copy, colors, and spacing.

Example (design spec for analysis — not a final asset):

```bash
curl -L -o assets/page.png "https://diffui.ai/image/Page_Name.webp?authToken=zaqPoXg9uAxI&format=png"
# or
curl -L -o assets/page.png "https://diffui.ai/image/Page_Name.png?authToken=zaqPoXg9uAxI"
```

Replace `Page_Name` with the slug from each design image URL in this document.

**Note:** Design image URLs are binary assets. URL-fetch-to-markdown tools may fail; download the file instead.


**Do not use design crops as final page assets.** Never crop a hero image, photo, or illustration out of the design reference and place that crop in your HTML/CSS. The design image is a **specification only** — generate real high-resolution assets via `build/generate-image` (see **Generating image assets** below). Cropping the design is not a substitute for generation.

The `&crop=x,y,width,height` query param is for **inspection and intermediate pipelines only** (e.g. measuring a region, or feeding a large illustration into image-to-SVG). It must **not** appear in any `<img src>`, CSS `background-image`, or other shipped asset path.

## Project setup

Inspect the target repository before choosing a stack:

| Situation | What to do |
|-----------|------------|
| **Existing project** | Use the framework, tooling, and patterns already present. Do not introduce a different stack. |
| **Empty repo + linked npm package** (see Brand context) | Scaffold the minimum project that package requires, install it, then implement. Do not default to vanilla HTML/CSS when a brand package is linked. |
| **Empty repo, no linked package** | Use vanilla HTML, CSS, and JavaScript. |

When a linked npm package implies a specific framework (React, Vue, etc.) or tooling (Tailwind, shadcn CLI), scaffold that stack with sensible defaults — e.g. Next.js or Vite for React-based design systems — even if this document does not spell out every init command.

## Implementation guidelines

You are an elite frontend engineer and design-to-code specialist. The design image is the primary source of truth; your code is the translation layer. Do not reinterpret or "improve" the design into something generic — reproduce it faithfully.

Before writing code, analyze the image like a design specification:

- **Layout & structure:** overall grid, section ordering, alignment, column logic, content width.
- **Typography:** extract visible text verbatim; size/weight hierarchy, display vs body contrast, line height, tracking, serif vs sans.
- **Spacing:** section padding, gutters, gaps, card padding, image-to-text distance. Preserve generous spacing; do not compress.
- **Color:** background, panels, accents, button fills, text hierarchy, borders, shadows. Preserve the exact palette you observe; do not substitute generic web colors.
- **Textures & surfaces:** Look for repeating photographic or organic surfaces used as tiled backgrounds — paper grain, linen, concrete, stone, wood grain, fabric weave, film noise, subtle marbling. Note tiling direction if visible: omnidirectional repeat → `both`; stripe or band patterns that repeat on one axis only → `horizontal` or `vertical`. Do **not** generate textures for flat solid fills, simple CSS gradients, hero photos, or illustrations — use CSS or `build/generate-image` instead. Only call `build/create-texture` when a seamless repeating surface is clearly part of the shipped design. Request extra PBR maps (`normal`, `roughness`, etc.) **only** when the user explicitly wants WebGL-style lighting or physically based shading — not for ordinary CSS `background-repeat` tiles.
- **Components:** buttons (shape, radius, fill vs outline, padding, primary/secondary), cards, inputs, badges, dividers, icons.
- **Imagery:** Match the container silhouette, not just the asset. Non-rectangular boundaries are first-class layout — implement with clip/mask, not border-radius. Generate high-res images via `build/generate-image`; generate seamless repeating surfaces via `build/create-texture` when the design calls for them; never ship crops from the design reference as final artwork.

Implementation discipline:

- Preserve layout logic, spacing rhythm, section ordering, text/image balance, typography mood, and component styling.
- Use the actual visible text from the image, not placeholder copy.
- Match colors and spacing to what you observe, not to defaults.
- Do not add nested box-in-box wrappers, decorative pills, fake status labels, or micro-UI clutter not in the image.
- Avoid AI-slop (default purple/blue gradients, glow, glassmorphism, generic card spam) unless the image clearly shows it.
- Keep the first viewport clean and readable; responsive in spirit while keeping the desktop composition faithful.
- Follow the **Project setup** rules above: match an existing repo's stack, scaffold for a linked npm package on an empty repo, or use vanilla HTML/CSS/JS when no package is linked.

Resolve ambiguity in this order: preserve the visible design language, then layout/spacing logic, then component family, then mood/polish — only then fall back to a faithful choice. The final result should look like the same design in the image, translated into real code.

## Generating image assets

When the design needs photos or illustrations (avatars, hero images, product shots, lineup photos, etc.), **generate them via Diffui** — do not use placeholders and **do not crop them out of the design reference**.


**Required for all photographic and illustrative content in the page:**

- Hero banners, scene illustrations, artist photos, product shots, and any other raster imagery → `build/generate-image`
- Use **high** quality + `referenceImageUrl` when recreating a large hero or focal illustration from the design (see **Reference images**).
- Use **medium** quality for lineup photos, avatars, and card imagery.
- If generation fails, **retry** (including waiting longer for high-quality hero requests). Do not fall back to a cropped region of the design image as the shipped asset.

**Endpoint:** `POST https://diffui.ai/api/build/generate-image`

**Headers:** `Content-Type: application/json`

**Body (JSON):**

```json
{
  "authToken": "zaqPoXg9uAxI",
  "prompt": "A portrait of a man for an avatar",
  "width": 512,
  "height": 512,
  "quality": "medium"
}
```

**Required fields:**

- `authToken` — the same token from this build link (`zaqPoXg9uAxI`).
- `prompt` — plain-text description of the image (describe what you want; no negative prompts).
- `width` and `height` — **required** on every request. Choose dimensions explicitly for each asset.

**Optional fields:**

- `quality` — `"medium"` (default) or `"high"`. See **Choosing quality** below.
- `referenceImageUrl` — a Diffui image URL to use as a visual reference. **High quality (`"high"` / gpt-image-2) only.** See **Reference images** below.

**Response (JSON):**

```json
{
  "url": "https://diffui.ai/image/generated_example.webp?authToken=zaqPoXg9uAxI",
  "width": 512,
  "height": 512
}
```

Download the returned `url` to a local file (see **Local assets**), then reference that path in your HTML/CSS. Do not hotlink the remote URL. The auth token expires after seven days; if generation fails with 403, ask the user to re-copy the build link from Diffui.

To inspect a generated asset with a vision tool, download PNG the same way as design images (`&format=png` or a `.png` path suffix).

### Choosing quality

Pick `quality` per asset. **Default to medium** — it is faster, cheaper (1¢), and sufficient for most page assets.

| Use **medium** (default, 1¢) | Use **high** (13¢) |
|------------------------------|---------------------|
| Profile pictures, avatars | Large hero / banner images |
| Product shots, thumbnails | Anything needing ~2K resolution |
| Icons, small decorative images | Illustrations, artistic focal imagery |
| Most page assets | Main visual focus of the page |

**Medium example** (avatar):

```json
{
  "authToken": "zaqPoXg9uAxI",
  "prompt": "Professional headshot portrait of a woman, neutral background",
  "width": 512,
  "height": 512,
  "quality": "medium"
}
```

**High example** (hero illustration):

```json
{
  "authToken": "zaqPoXg9uAxI",
  "prompt": "Wide cinematic illustration of a mountain landscape at sunset, painterly style",
  "width": 2048,
  "height": 1536,
  "quality": "high"
}
```

### Reference images (high quality only)

Use `referenceImageUrl` **only** when you need gpt-image-2 to recreate a **standalone background or illustration** that appears inside the design — not the full page screenshot.

**Use a reference image when:**

- The design shows a large hero illustration, scene, or photographic background that must become a real `<img>` or CSS background asset.
- You need a **full-resolution, isolated** version of that artwork (no navigation, buttons, headlines, or other UI chrome).
- A plain-text prompt alone would not preserve the composition, subjects, palette, or graphic details visible in the design.

**Do not use a reference image when:**

- Generating avatars, product shots, icons, thumbnails, or other assets that are **not** copied from a specific region of the design image.
- The asset is simple enough to describe accurately with `prompt` alone.
- You want to regenerate the **entire page** or any layout that includes UI elements — implement those in HTML/CSS from the design reference instead.

Pass a Diffui URL from this document (design image or a previously generated asset). The `authToken` in `referenceImageUrl` must match `authToken` in the request body. `referenceImageUrl` is rejected unless `quality` is `"high"`.

**High example** (isolated hero illustration from the design):

```json
{
  "authToken": "zaqPoXg9uAxI",
  "prompt": "Recreate only the hero illustration from the reference: saxophonist in black suit and fedora against a New Orleans skyline, yellow sun disc, expressive JAZZ lettering, and purple brushstroke banner — no navigation, buttons, headlines, or page chrome. Match the reference composition, subjects, and color palette.",
  "width": 2048,
  "height": 1536,
  "quality": "high",
  "referenceImageUrl": ""
}
```

### Generation timing

- Each image request typically takes **around 60 seconds**; occasionally one can take **up to 3 minutes**.
- Wait for each request to finish before using its returned `url`.
- When you need **multiple assets**, send **separate requests in parallel** whenever you can (concurrent POSTs with different prompts and dimensions). Parallel generation is much faster than generating images one at a time.

### Medium quality (z-image) size rules

Used when `quality` is `"medium"` or omitted. Every `width`/`height` pair must satisfy all of:

| Constraint | Value |
|------------|-------|
| Minimum edge | 512 px |
| Maximum edge | 2048 px |
| Maximum aspect ratio | 3:1 (neither side more than 3× the other) |

**Sizing guidance:**

- **Avatars / icons / small squares:** **512×512**
- **Product shots / cards:** **512×768** or **768×512**
- **Medium landscape panels:** **1024×768**

Invalid dimensions return HTTP 400 with an error message.

### High quality (gpt-image-2) size rules

Used when `quality` is `"high"`. Every `width`/`height` pair must satisfy all of:

| Constraint | Value |
|------------|-------|
| Minimum edge | 16 px |
| Maximum edge | 2048 px |
| Multiple of | 16 |
| Maximum aspect ratio | 3:1 (neither side more than 3× the other) |
| Minimum total pixels | 655,360 |

**Sizing guidance:**

- **Avatars / icons / small squares:** use **816×816** (smallest valid square).
- **Landscape hero / banner:** **2048×1536**
- **Portrait hero:** **1536×2048**
- **Wide thumbnails:** e.g. **1280×512** (must still meet the pixel minimum)

Invalid dimensions return HTTP 400 with an error message. There is no default size — always specify `width` and `height` intentionally for each asset.

## Seamless background textures

When the design uses a **repeating photographic or organic surface** as a background (paper grain, linen, concrete, stone, fabric weave, subtle noise overlay), generate a seamless tile via Diffui — do not approximate with a cropped region from the design reference and do not use `build/generate-image` for textures.

**Generate a texture when:**

- The design shows a surface meant to tile with `background-repeat` (paper, linen, concrete, fabric, stone, film grain, etc.)
- A plain CSS color or gradient cannot faithfully reproduce the visible surface

**Do not generate a texture when:**

- The background is a flat solid color or a simple linear/radial gradient → use CSS
- The surface is glass, blur, or frosted effects → use CSS `backdrop-filter` or similar
- The asset is a hero photo, illustration, or product shot → use `build/generate-image`
- The pattern is a standard UI motif achievable with CSS alone

**Endpoint:** `POST https://diffui.ai/api/build/create-texture`

**Headers:** `Content-Type: application/json`

**Body (JSON):**

```json
{
  "authToken": "zaqPoXg9uAxI",
  "prompt": "warm off-white paper grain, subtle fiber texture",
  "tilingMode": "both"
}
```

**Required fields:**

- `authToken` — the same token from this build link (`zaqPoXg9uAxI`).
- `prompt` — describe the surface or material only (what you want the tile to look like).

**Optional fields:**

- `tilingMode` — `"both"` (default), `"horizontal"`, or `"vertical"`. Use `"both"` for omnidirectional repeats. Use `"horizontal"` or `"vertical"` only when the design clearly repeats on one axis (e.g. horizontal wood slats or vertical fabric stripes).
- `maps` — optional array of PBR map types to generate alongside the seamless tile. Allowed values: `"basecolor"`, `"normal"`, `"roughness"`, `"metalness"`, `"height"`. Omit it or pass an empty array to use the Fal model default. **Only request specific maps when the user explicitly wants WebGL-style lighting or physically based shading** — not for standard CSS repeating backgrounds.

Example with a normal map for a lit 3D surface:

```json
{
  "authToken": "zaqPoXg9uAxI",
  "prompt": "weathered brushed steel, fine directional scratches",
  "tilingMode": "both",
  "maps": ["normal", "roughness"]
}
```

**Response (JSON):**

```json
{
  "url": "https://diffui.ai/image/generated_texture.webp?authToken=zaqPoXg9uAxI",
  "width": 1024,
  "height": 1024,
  "tilingMode": "both",
  "maps": [
    {
      "type": "normal",
      "url": "https://diffui.ai/image/generated_texture_normal.webp?authToken=zaqPoXg9uAxI"
    },
    {
      "type": "roughness",
      "url": "https://diffui.ai/image/generated_texture_roughness.webp?authToken=zaqPoXg9uAxI"
    }
  ]
}
```

When `maps` is omitted, Diffui omits the field from the Fal request so the model uses its default maps. If Fal returns map images, Diffui includes them in the `maps` response array.

Output is always **1024×1024**. Download the returned `url` locally (see **Local assets**), then reference that path in CSS:

| `tilingMode` | CSS `background-repeat` |
|---------------------|--------------------------------|
| `both` | `repeat` |
| `horizontal` | `repeat-x` |
| `vertical` | `repeat-y` |

- Cost: **3¢** per call for the seamless tile alone; **+1¢** per additional PBR map at 1024×1024 (e.g. tile + normal = 4¢).
- Typical workflow: identify the surface during design analysis → generate once → apply as a repeating CSS background. Request extra maps only for WebGL / Three.js materials with real-time lighting.

## Removing image backgrounds

When you need a transparent cutout (product on white, profile photo, logo), remove the background via Diffui after generating or sourcing the image. The endpoint accepts either a remote image URL (JSON) or a **local file uploaded directly** (multipart) — use the upload form for user-provided images that only exist on disk.

**Endpoint:** `POST https://diffui.ai/api/build/remove-background`

### Option A — remote image URL (JSON)

**Headers:** `Content-Type: application/json`

**Body (JSON):**

```json
{
  "authToken": "zaqPoXg9uAxI",
  "imageUrl": "https://diffui.ai/image/generated_example.webp?authToken=zaqPoXg9uAxI"
}
```

- `imageUrl` — a publicly fetchable image URL. Use a Diffui asset URL from `build/generate-image` (include the `authToken` query param).

### Option B — local file upload (multipart)

Send the file bytes directly as `multipart/form-data` — no hosting required:

```bash
curl -X POST "https://diffui.ai/api/build/remove-background" \
  -F "authToken=zaqPoXg9uAxI" \
  -F "file=@assets/product-photo.png"
```

- `file` — the image file (PNG, JPEG, or WebP; max 25MB).
- Pass exactly one of `imageUrl` or `file`, not both.

**Required in both forms:**

- `authToken` — the same token from this build link.

**Response (JSON):**

```json
{
  "url": "https://diffui.ai/image/generated_cutout.webp?authToken=zaqPoXg9uAxI",
  "width": 512,
  "height": 512
}
```

- Cost: **1¢** per call.
- Typical workflow: generate with **medium** quality → remove background for product/profile cutouts.
- Download the returned `url` locally before using it in code (see **Local assets**).
- The returned image is a transparent PNG served as WebP for display; use `&format=png` when you need the alpha channel for inspection.

## Generating PBR maps from an image

When an existing image needs WebGL-style lighting — normal, height (displacement), roughness, basecolor (albedo), or metalness maps — generate the maps via Diffui. Use this for hero artwork, product cutouts, or any raster that will be lit in a shader. **Only call it when the user explicitly wants physically based lighting or displacement**, not for ordinary flat imagery.

Like remove-background, the input is either a remote URL (JSON) or a **local file uploaded directly** (multipart).

**Endpoint:** `POST https://diffui.ai/api/build/create-maps`

### Option A — remote image URL (JSON)

**Headers:** `Content-Type: application/json`

**Body (JSON):**

```json
{
  "authToken": "zaqPoXg9uAxI",
  "imageUrl": "https://diffui.ai/image/generated_example.webp?authToken=zaqPoXg9uAxI",
  "maps": ["normal", "height", "roughness"]
}
```

### Option B — local file upload (multipart)

```bash
curl -X POST "https://diffui.ai/api/build/create-maps" \
  -F "authToken=zaqPoXg9uAxI" \
  -F "file=@assets/card.png" \
  -F "maps=normal,height,roughness"
```

**Required fields:**

- `authToken` — the same token from this build link.
- Exactly one of `imageUrl` (publicly fetchable URL) or `file` (PNG, JPEG, or WebP; max 25MB).

**Optional fields:**

- `maps` — which maps to predict: `"basecolor"`, `"normal"`, `"roughness"`, `"metalness"`, `"height"`. Omit to generate **all five**. Request only the maps your shader actually samples — cost scales per map.

**Response (JSON):**

```json
{
  "width": 1024,
  "height": 1536,
  "maps": [
    {
      "type": "normal",
      "url": "https://diffui.ai/image/generated_example_normal.webp?authToken=zaqPoXg9uAxI"
    },
    {
      "type": "height",
      "url": "https://diffui.ai/image/generated_example_height.webp?authToken=zaqPoXg9uAxI"
    },
    {
      "type": "roughness",
      "url": "https://diffui.ai/image/generated_example_roughness.webp?authToken=zaqPoXg9uAxI"
    }
  ]
}
```

Maps match the input image dimensions and share its UV space — sample them with the same coordinates as the source texture.

- Cost: **1¢** base + **1¢ per megapixel per map** (e.g. a 1024×1024 input with all 5 maps = 6¢).
- Download each returned map `url` locally before referencing it (see **Local assets**); use `&format=png` when the shader pipeline needs lossless data.
- Unlike `build/create-texture` (which *generates a new seamless tile* from a text prompt), `build/create-maps` derives maps for an image you already have.

## Generating SVG assets

**Endpoint:** `POST https://diffui.ai/api/build/generate-svg`

**Headers:** `Content-Type: application/json`

Pick the mode by asset type — **do not use image-to-SVG for small UI icons**; vectorizing cropped icon pixels produces poor results. Use **text-to-SVG** for icons instead.

### Custom icons → text-to-SVG only

Use Quiver **text-to-SVG** for **non-standard icons** not available in your project's icon library — for example a ferris wheel for a county fair or a unique pictogram from the design.

**Do not generate SVGs for standard UI icons** (arrows, chevrons, menu, close, search, social logos, etc.). Use the icon pack your project already uses (Lucide, Heroicons, Phosphor, Material Symbols, etc.).

**Do not use image-to-SVG for icons** — even with a tight crop from the design, results are usually unusable. Describe the icon in a `prompt` instead.

**Before generating:** search the icon pack for a close match. Only call `build/generate-svg` when no suitable icon exists.

Describe the icon in `prompt`. Optional `instructions` sets style (flat monochrome, line weight, etc.). **Always set `viewBox`** so the artwork fits the canvas and is not clipped. Use a square viewBox for icons (e.g. 64×64).

```json
{
  "authToken": "zaqPoXg9uAxI",
  "prompt": "Minimal ferris wheel icon for a county fair app, single-color line art",
  "instructions": "Flat monochrome icon. Draw entirely inside the viewBox with comfortable padding; do not extend past the canvas edges.",
  "viewBox": { "minX": 0, "minY": 0, "width": 64, "height": 64 }
}
```

### Complex illustrations → image-to-SVG only

Use **image-to-SVG** only for **large or complex artwork** that should become a vector illustration — for example a detailed logo mark, badge, or decorative graphic region from the design. **Not for footer icons, toolbar glyphs, or other small UI pictograms.**

1. Download the design as PNG (`&format=png`) and inspect it.
2. Measure the illustration's bounding box in pixels (not a tiny icon crop).
3. Fetch a crop around that region: `&format=png&crop=x,y,width,height` — no labels or neighboring UI unless they are part of the artwork.
4. Pass that URL as `imageUrl`. Use a viewBox that matches the illustration's aspect ratio and scale (e.g. 512×512 for a square logo).

```json
{
  "authToken": "zaqPoXg9uAxI",
  "imageUrl": "https://diffui.ai/image/Hero.webp?authToken=zaqPoXg9uAxI&format=png&crop=400,80,640,480",
  "viewBox": { "minX": 0, "minY": 0, "width": 512, "height": 384 }
}
```

**Required fields:**

- `authToken` — the same token from this build link.
- Exactly one of `prompt` (text-to-SVG, **icons only**) or `imageUrl` (image-to-SVG, **illustrations only**).
- `viewBox` — SVG canvas forwarded to Quiver as `attributes.viewBox`. Set `minX` and `minY` to `0` unless you have a reason not to. Artwork must fit entirely inside this box with padding.

**Optional field:**

- `instructions` — text-to-SVG only; style guidance for Quiver. Mention that the icon must stay inside the viewBox.

**Response (JSON):**

```json
{
  "url": "https://diffui.ai/image/generated_example.svg?authToken=zaqPoXg9uAxI",
  "width": 24,
  "height": 24
}
```

- Cost: **2¢** per SVG.
- Download the returned `url` locally before using it inline or as `<img src>` (see **Local assets**).

