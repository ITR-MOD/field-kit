# UNPSC UI Theme & Design Standards
**Into The Radius 2 — Inspired Design System**

> Reference document for agent-assisted UI construction. All decisions here are derived from observable in-game assets, lore context, official developer communications, and the overarching aesthetic logic of the UNPSC faction as presented in ITR2.

---

## 1. Faction Identity & Aesthetic Context

**UNPSC** (United Nations Pechorsk Special Committee) is a bureaucratic-military organization operating at the edge of a paranormal anomaly zone. Their visual language sits at the intersection of:

- **Cold War / post-Soviet institutional design** — functional, utilitarian, authoritarian. Think Soviet-era scientific agency aesthetics crossed with UN peacekeeping beige.
- **Operational military field kit** — ruggedized, practical, no decorative flourishes. Equipment labels, stencil fonts, warning markings.
- **Isolated frontier science outpost** — low-resource, improvised but organized. Fluorescent lighting, metal shelving, laminated paper signs.
- **Bureaucratic legitimacy** — forms, terminals, mission dossiers, supply credits. Everything is logged, stamped, tracked.

The UNPSC presents a veneer of professionalism over a morally ambiguous operation. Their UI should feel *official but not polished* — like a government contractor's internal software from the early 2000s, designed for function over experience.

---

## 2. Color Palette

### Primary Colors

| Role | Name | Hex | Notes |
|---|---|---|---|
| Background | Deep Charcoal | `#1A1C1E` | Near-black, slight warm-grey lean. Never pure black. |
| Surface / Panel | Dark Steel | `#252829` | Card/panel backgrounds. |
| Surface Raised | Gunmetal | `#2E3235` | Elevated panels, dropdowns. |
| Border / Divider | Cold Iron | `#3D4347` | Hairline borders, grid lines. |

### Accent Colors

| Role | Name | Hex | Notes |
|---|---|---|---|
| Primary Accent | UNPSC Teal | `#4ECDC4` | CRT-green adjacent. Used for active states, highlights, key data. |
| Secondary Accent | Amber Alert | `#F4A72A` | Warnings, mission priority indicators, status alerts. |
| Danger / Alert | Anomaly Red | `#C0392B` | Critical failures, hostile indicators, danger zones. |
| Success / Clear | Safe Green | `#27AE60` | Completed missions, safe status, confirmed actions. |
| Disabled / Inactive | Ash | `#5A6068` | Grayed-out elements, locked content. |

### Text Colors

| Role | Hex | Usage |
|---|---|---|
| Primary Text | `#E8EAE8` | Body copy, labels — slightly warm white, not pure |
| Secondary Text | `#9BA3A8` | Descriptions, metadata, secondary labels |
| Accent Text | `#4ECDC4` | Active links, selected states, highlighted values |
| Warning Text | `#F4A72A` | Mission risk levels, caution notices |
| Disabled Text | `#4D5359` | Inactive, locked, unavailable content |

### Color Philosophy
- **Dark-first.** All surfaces are dark. There are no light-mode states.
- **Low saturation base, high saturation accents.** The base palette is near-monochromatic; accents punch hard.
- **CRT ghost.** Teal/green accents reference old computer terminals and military hardware displays. The UNPSC's tech isn't cutting-edge — it's dependable, institutional, slightly dated.
- **Amber as priority signal.** Amber/orange is reserved for things the user must notice: mission urgency, alerts, warnings. Never use it decoratively.

---

## 3. Typography

### Font Stack

**Display / Headers:** `"Share Tech Mono"` or `"Courier Prime"` (monospace, military stencil feel)
- Used for: terminal headers, mission IDs, security level labels, facility designations
- Character: Typewriter/terminal. Evokes stenciled equipment labels and dot-matrix printouts.

**UI Labels / Navigation:** `"Barlow Condensed"` or `"Oswald"` (condensed sans-serif)
- Used for: button text, nav items, category labels, table headers
- Character: Compressed, authoritative. Reads like military form field labels.

**Body / Descriptions:** `"Roboto"` or `"Source Sans 3"` (readable sans-serif)
- Used for: mission descriptions, item info, longer text blocks
- Character: Utilitarian clarity. Not decorative — purely readable.

**Data / Stats:** `"Share Tech Mono"` or `"JetBrains Mono"` (monospaced)
- Used for: supply credit balances, weight values, coordinates, dates, item counts
- Character: Data terminal output. Always monospaced for alignment.

### Type Scale

| Level | Size | Weight | Font | Usage |
|---|---|---|---|---|
| Display | 28–32px | 700 | Condensed | Page/section titles |
| Heading | 18–22px | 600 | Condensed | Panel headers, category names |
| Subheading | 14–16px | 500 | Condensed | Sub-labels, column headers |
| Body | 13–15px | 400 | Body sans | Descriptions, info blocks |
| Caption | 11–12px | 400 | Body sans | Metadata, timestamps, footnotes |
| Data | 12–14px | 400 | Mono | Stats, values, IDs |

### Typography Rules
- **ALL CAPS** for category headers, navigation items, and security level labels. This is a deliberate institutional/military convention.
- Monospace for all numerical values (credits, weight, coordinates) — ensures alignment in tables.
- Minimal use of italic; bold used only for critical priority.
- Letter-spacing: +0.05em to +0.1em on ALL CAPS display labels.

---

## 4. Spacing & Layout

### Grid System
- **Base unit:** 4px
- **Component padding:** 12–16px internal
- **Panel gap:** 8–12px between sibling panels
- **Section gap:** 24–32px between major sections

### Layout Principles
- **Dense but not cramped.** ITR2's UI is field equipment — it packs information efficiently. Aim for information density similar to a military inventory form.
- **Left-aligned data.** All primary data reads left-to-right, top-to-bottom. No centered body text.
- **Structured columns.** Use grid/table layouts for mission lists, item lists, inventory. Irregular/freeform layouts are not appropriate.
- **Panel hierarchy.** Three levels: background → panel → raised element. No more depth than this.

---

## 5. Component Standards

### Panels & Cards
- Background: `#252829` (surface)
- Border: 1px solid `#3D4347`
- Border-radius: **0–2px only.** UNPSC hardware has squared corners. No rounded cards.
- Optional: thin top border accent in `#4ECDC4` for "active" or "selected" panels.
- Shadow: Minimal. `0 2px 8px rgba(0,0,0,0.4)` at most. Not decorative.

### Buttons

**Primary (Action):**
- Background: `#4ECDC4` at 15–20% opacity, border: 1px solid `#4ECDC4`
- Text: `#4ECDC4` in ALL CAPS condensed font
- Hover: Fill to solid `#4ECDC4`, text to `#1A1C1E`
- No border-radius.

**Secondary:**
- Background: transparent, border: 1px solid `#3D4347`
- Text: `#9BA3A8`
- Hover: border color to `#5A6068`, text to `#E8EAE8`

**Danger/Confirm:**
- Background: `#C0392B` at 15% opacity, border: `#C0392B`
- Text: `#C0392B`

**Disabled:**
- All elements shift to `#3D4347` border, `#4D5359` text. No interaction.

### Input Fields
- Background: `#1A1C1E` (recessed, one level darker than surface)
- Border: 1px solid `#3D4347`
- Focus border: `#4ECDC4`
- Text: `#E8EAE8`, placeholder: `#4D5359`
- Border-radius: 0px
- No floating labels. Labels are always above the field, uppercase, small.

### Tables & Lists
- Alternating rows: surface `#252829` / slightly lighter `#2A2D30`
- Row hover: `#2E3235` with left border `2px solid #4ECDC4`
- Column headers: ALL CAPS, `#9BA3A8`, 11px, tracking +0.1em
- Dividers: `#3D4347` at 1px

### Status Indicators / Badges
- Always use a small filled circle + label pattern
- Colors map directly to accent palette:
  - Active/Safe: `#27AE60` dot
  - Warning/Priority: `#F4A72A` dot
  - Critical/Hostile: `#C0392B` dot
  - Inactive/Locked: `#5A6068` dot
- Shape: square badge (0 border-radius), small — never pill-shaped

### Progress Bars / Meters
- Track: `#2E3235` (dark)
- Fill: gradient from `#4ECDC4` to `#27AE60` (health/condition) or `#F4A72A` to `#C0392B` (danger states)
- Height: 4–6px. Thin and utilitarian.
- No border-radius on track or fill.

### Icons
- Line-art style, 16–20px. Clean, functional.
- Prefer iconography sourced from: military/survival contexts (crosshairs, shields, mission markers, weight scales, supply crates)
- Color: matches surrounding text (`#9BA3A8` default, `#4ECDC4` for active/interactive)
- No decorative illustration. Icons are functional signals only.

---

## 6. In-Game UI Reference Elements

### The Explorer's Tablet
The player's primary in-game UI surface. Key characteristics:
- Dark matte screen with slight glare/scan-line texture
- Map view: topographic, low-color, functional — not stylized
- Text overlays: monospace, small, tight margins
- UI chrome on the tablet itself: physical buttons, corner grips — ruggedized military PDA aesthetic
- Can be spray-painted in-game → the shell is separate from the screen UI

### The Command Room Terminal
The mission selection and shop interface in Facility 27:
- Physical terminal (not flat screen) — implies CRT monitor era or ruggedized LCD
- Mission listings: structured list with priority tiers, location identifiers, reward breakdowns
- Security Level indicator: prominent numerical display
- "Supply Depot" functions: item grid with supply credit costs, simple text descriptions

### Facility 27 Physical Design Language
Informs UI tone:
- Concrete, metal, weatherproofed materials
- Institutional lighting (cool fluorescent, not warm)
- Stenciled text on surfaces (room labels, hazard markings)
- UN-blue accents mixed with Soviet utilitarian grey/green
- Mission briefing room: paper documents, corkboard with map markers — analog crossed with digital

---

## 7. Motion & Animation

**Guiding principle: This is utilitarian hardware, not a polished consumer app.** Animations exist to communicate state, not to delight.

- **No easing curves that feel "designed."** Use `ease-in-out` or `linear` only.
- **Transition duration:** 80–150ms for state changes. Never slow fades on interactive elements.
- **Acceptable animations:**
  - Quick opacity transitions on hover (80ms)
  - Slide-in for panels from left/top (150ms, no spring/bounce)
  - Blinking cursor in text inputs or terminal displays
  - CRT scan-line flicker effect on headers (subtle, CSS-only, optional)
  - Loading states: simple horizontal progress bar, not spinners
- **No:** bounce, spring physics, parallax, floating elements, particle effects

---

## 8. Writing & Tone

All copy should feel like it was written by a mid-level UN bureaucrat managing a classified operation:

- **Terse and transactional.** No warmth. No brand voice. No "Hey there!"
- **Mission language:** "OBJECTIVE ASSIGNED," "AWAITING CONFIRMATION," "SUPPLY CREDIT BALANCE," "SECURITY CLEARANCE LEVEL"
- **Numbered identifiers everywhere:** Explorer #73, Facility 27, Mission Ref: TPRIO-004
- **Passive voice for system messages:** "Mission objective updated." "Item transferred to tray."
- **Abbreviate naturally:** SC (Supply Credits), SL (Security Level), UNPSC, ITR
- **Error states:** Clinical. "ACTION FAILED. REASON: INSUFFICIENT CLEARANCE."
- **No emojis, no decorative punctuation.**

---

## 9. Texture & Atmosphere (CSS/Shader Guidance)

To be applied selectively, not universally:

```css
/* CRT scan-line overlay — apply to terminal/header surfaces */
background-image: repeating-linear-gradient(
  to bottom,
  transparent 0px,
  transparent 1px,
  rgba(0,0,0,0.08) 1px,
  rgba(0,0,0,0.08) 2px
);

/* Noise texture grain — add depth to panel backgrounds */
/* Use a base64-embedded SVG noise filter or CSS filter */
filter: url(#noise); /* paired with an SVG noise filter in defs */

/* Vignette on full-screen overlays */
background: radial-gradient(
  ellipse at center,
  transparent 60%,
  rgba(0,0,0,0.5) 100%
);

/* Hairline accent on active panel top edge */
border-top: 2px solid #4ECDC4;
```

**Texture rules:**
- Scan-lines: terminal/readout areas only
- Grain/noise: panel backgrounds to break flat surfaces
- No bloom or glow effects — too sci-fi, wrong tone
- Subtle vignette on modal overlays

---

## 10. Anti-Patterns (Do NOT)

| ❌ Don't | ✅ Do Instead |
|---|---|
| Rounded cards (8px+ radius) | Square corners (0–2px max) |
| Gradient mesh backgrounds | Flat dark surface + noise texture |
| Inter / Roboto as display font | Condensed militaristic or monospace display |
| Purple/blue gradients | Dark steel base + teal/amber accents |
| Soft shadows for depth | Hard 1px borders for definition |
| Animated spinners | Linear progress bars |
| Centered hero text | Left-aligned, structured data layouts |
| Friendly/conversational copy | Terse bureaucratic operational language |
| Pill-shaped buttons/badges | Rectangular (squared) buttons/badges |
| Icons with fills/gradients | Flat line-art icons |
| Light mode variant | Dark only |

---

## 11. Quick Reference: Design Token Summary

```json
{
  "color": {
    "bg": "#1A1C1E",
    "surface": "#252829",
    "surfaceRaised": "#2E3235",
    "border": "#3D4347",
    "accentPrimary": "#4ECDC4",
    "accentWarning": "#F4A72A",
    "accentDanger": "#C0392B",
    "accentSuccess": "#27AE60",
    "textPrimary": "#E8EAE8",
    "textSecondary": "#9BA3A8",
    "textDisabled": "#4D5359"
  },
  "font": {
    "display": "'Share Tech Mono', monospace",
    "ui": "'Barlow Condensed', 'Oswald', sans-serif",
    "body": "'Roboto', 'Source Sans 3', sans-serif",
    "data": "'JetBrains Mono', 'Share Tech Mono', monospace"
  },
  "radius": {
    "component": "0px",
    "max": "2px"
  },
  "spacing": {
    "base": "4px",
    "componentPad": "12px",
    "panelGap": "8px",
    "sectionGap": "24px"
  },
  "border": "1px solid #3D4347",
  "transitionFast": "80ms ease-in-out",
  "transitionNormal": "150ms ease-in-out"
}
```

---

*Document prepared for agent handoff. The above tokens, rules, and references should be sufficient to build any UNPSC-themed UI component with correct aesthetic fidelity to ITR2's faction visual language.*
