# Flux UI/UX Design Philosophy

> Design reference for the Flux Web UI. Covers current system, evaluated alternatives, and chosen direction.

---

## Current Design System (Post-Phase 2B)

### Core Principles

1. **Semantic color tokens, not raw values.** Components use `text-content-secondary`, `bg-surface-hover`, `border-line` — never `text-gray-600` or `bg-white`. Swapping `data-theme` changes the entire palette without touching components.

2. **Card-based layout with subtle depth.** `.card` class: white surface, 1px border, very light box-shadow (`rgba(0,0,0,0.04)`). Hover adds slightly deeper shadow. No heavy gradients or glassmorphism.

3. **Compact, information-dense UI.** Small text sizes (`text-[11px]`, `text-[13px]`, `text-sm`), tight spacing, uppercase micro-labels (`tracking-wider`), tabular numbers for data. Designed for an operator monitoring a system, not a consumer product.

4. **Frosted sidebar with collapse.** `backdrop-blur-xl`, semi-transparent background (`bg-sidebar/50`), collapsible to 60px icon-only mode. Mobile: slides in with overlay.

5. **Inter + JetBrains Mono.** System-like sans-serif for UI, monospace for code/data. No decorative fonts.

6. **Touch-ready with accessibility.** 44px minimum touch targets (`min-h-[44px]`), `touch-manipulation` everywhere, skip-to-content link, `aria-label` on icon-only buttons, `aria-pressed` on theme selector.

7. **Restrained motion.** `fade-in`, `slide-up`, `slide-in-right` — all 200-300ms, `ease-out`. No bouncy or attention-grabbing animations. `glow-pulse` and `shimmer` reserved for loading states only.

8. **Component classes in CSS, not utilities-only.** `.btn-primary`, `.btn-sm`, `.badge-success`, `.input`, `.label` — reusable Tailwind `@apply` compositions in `index.css`. Components reference these classes, keeping TSX clean.

9. **No external component library.** No shadcn, no Radix, no MUI. Everything is hand-built with Tailwind. `recharts` is the only planned charting dependency.

### Point Color Scheme — Sage Green & Light Blue

Light-mode only. Two point color themes switchable via sidebar dot selector. Each theme tints the entire UI — page background, sidebar, borders, primary actions, badges, and focus rings all shift to match.

| Theme | Name | Point Color | Primary (`p500`) | Page BG | Sidebar |
|-------|------|-------------|-------------------|---------|---------|
| `blue` | **Light Blue** | Periwinkle cobalt | `#4B6EF5` | `#F7F9FE` | `#F0F4FB` |
| `green` | **Sage Green** | Sage emerald | `#1DB960` | `#F5FAF6` | `#EDF5EF` |

**Color scale per theme** (50–700, used for backgrounds through text):

| Token | Light Blue | Sage Green | Usage |
|-------|-----------|------------|-------|
| `p50` | `#EFF4FF` | `#EFFDF4` | Selected row bg, hover tint |
| `p100` | `#DBE6FE` | `#D4F7E2` | Active filter bg, badge bg |
| `p200` | `#BFCFFD` | `#ABF0C8` | Focus ring, light accent |
| `p300` | `#93B1FC` | `#73E2A3` | Gradient end, decorative |
| `p400` | `#6B8CF8` | `#3ECF80` | Hover state |
| `p500` | `#4B6EF5` | `#1DB960` | **Primary action** — buttons, links, active nav |
| `p600` | `#3654E3` | `#12964D` | Button bg, strong accent |
| `p700` | `#2C43C9` | `#0F783F` | Text on light bg, pressed state |

**Semantic mapping** (same for both themes):
- Primary buttons → `p600` bg, `p500` hover
- Active nav item → `p50` bg, `p700` text
- Badge primary → `p50` bg, `p700` text, `p200/60` ring
- Focus ring → `p500/30`
- Selection → `p500` at 15% opacity
- Shadow → `p600` at 8% opacity
- Links → `p600` text, `p500` hover
- Inline code → `p50` bg, `p700` text

**Status colors** (theme-independent, always the same):
- Success: `emerald-600` — task completed, PR merged, tests passed
- Danger: `rose-600` — task failed, critical alert
- Warning: `amber-600` — rate limit, degraded state
- Info: `cyan-600` — running state, informational badge
- Purple: `violet-600` — decomposed tasks, special states

All colors are CSS custom properties (`--c-page`, `--c-surface`, `--c-text`, `--c-p500`, etc.) mapped through Tailwind config. The `data-theme` attribute on `<html>` controls the active palette. Persisted in `localStorage` under `flux-theme`.

### Component Inventory

| Class | Purpose |
|-------|---------|
| `.card` | White surface, 1px border, subtle shadow, hover lift |
| `.btn` / `.btn-sm` | Base button with 44px min-height, focus ring |
| `.btn-primary` / `.btn-secondary` / `.btn-danger` / `.btn-success` / `.btn-warning` | Semantic button variants |
| `.badge` / `.badge-success` / `.badge-info` / etc. | Status pills (11px, uppercase, ring border) |
| `.input` | Form input with focus ring transition |
| `.label` | Uppercase micro-label (10-11px, tracking-wider) |
| `.btn-filter` / `.btn-filter-active` / `.btn-filter-inactive` | Filter toggle buttons |
| `.prose-markdown` | Markdown rendering styles (headings, lists, code, tables) |

---

## Evaluated Design Philosophies

### 1. Linear Style — "Dark engineering aesthetic"

The dominant trend in developer/SaaS tools (Linear, Raycast, Vercel, Arc). Dark backgrounds, Inter font, gradient accents, glassmorphism, micro-animations.

| Aspect | Flux Current | Linear Style |
|--------|-------------|--------------|
| Background | Light (white surfaces, `#F7F9FE` page) | Dark (`#0A0A0B` page, `#1A1A1E` surfaces) |
| Accent | Periwinkle/sage theme swap | Gradient purple/violet with glow effects |
| Depth | Subtle box-shadow | Glassmorphism + backdrop-blur on everything |
| Motion | Restrained (200-300ms ease) | Micro-motion, gradient streamers, glow-pulse |
| Typography | Inter 13px compact | Inter 14-15px with bolder weights |
| Mood | Calm, administrative | Polished, cinematic |

**What makes it work**: Engineers love it. Feels premium. Eye comfort for long sessions. Inter on dark backgrounds mirrors coding environments. LCH color space enables perceptually uniform custom themes.

**Tradeoffs**: Homogeneous — every SaaS tool looks the same now. Flux's current light mode is actually more distinctive. Dark mode doubles the color token maintenance burden.

**Verdict**: Skip full dark rebrand. The light dual-theme is more distinctive than yet another Linear clone.

---

### 2. Bento Grid — "Modular compartments"

Apple-popularized layout. Content organized into asymmetric rectangular blocks of varying sizes. Used by Apple, Notion, many modern SaaS dashboards.

| Aspect | Flux Current | Bento Grid |
|--------|-------------|------------|
| Layout | Traditional sidebar + content column | Asymmetric grid tiles (hero tile + small tiles) |
| Cards | Uniform-height stat cards in a row | Mixed-size: large hero card, medium detail, small metric |
| Hierarchy | Size-uniform, position-based | Size-driven — bigger tile = more important |
| Spacing | Standard padding | Consistent 16-24px gaps, max 4-8 tiles per view |
| Scanning | Top-to-bottom list | Visual scan pattern follows tile size hierarchy |

**What makes it work**: Better visual hierarchy. Dashboard feels more alive. Each tile is a self-contained unit. Studies show 30% engagement increase. Minimalism reduces cognitive load — "distraction-free zones" for rapid content scanning.

**Implementation guidelines**:
- Content mapping: prioritize elements, sketch layouts before coding
- Consistent 16-24px gaps between tiles
- Limit to 4-8 compartments per view to prevent overcrowding
- Use CSS Grid (`grid-template-columns`) for responsive structure
- Each tile serves exactly one purpose (metric, chart, list, or status)

**Tradeoffs**: Harder to maintain responsiveness. Content must fit tile constraints. Risk of feeling "marketing page" instead of "ops dashboard."

**Verdict**: Adopt for Dashboard page. Replace `StatCard` row + lists with mixed-size tiles where the active Goal gets a hero tile and metrics get smaller tiles. Keep traditional list layout for Tasks, PRs, Projects pages.

---

### 3. Keyboard-First / Command Palette — "Power user efficiency"

Raycast, Linear, Vercel, Notion. Global `Cmd+K` palette for instant navigation, actions, and search without touching the mouse.

| Aspect | Flux Current | Keyboard-First |
|--------|-------------|----------------|
| Navigation | Mouse-click sidebar | `Cmd+K` → type "tasks" → enter |
| Actions | Button clicks | `Cmd+K` → "create task" → inline form |
| Search | No global search | Fuzzy search across tasks, projects, goals |
| Shortcuts | None | `G T` (go tasks), `G D` (go dashboard), `N T` (new task) |

**What makes it work**: Dramatically faster for an operator who uses the UI daily. Feels native/professional. Raycast CEO: "Linear is built for efficiency which is so important for developers. It doesn't get in the way."

**Implementation**: A single `CommandPalette.tsx` component with `Cmd+K`. Libraries like `cmdk` (by Pacocoursey/Vercel) make this trivial — ~3KB, headless, accessible out of the box.

**Tradeoffs**: Discoverability for new users. Flux has a single operator, so discoverability is a non-issue.

**Verdict**: Adopt. Highest ROI addition for Flux. Single component, massive daily impact.

---

### 4. Data Storytelling — "Narrative dashboards"

Grafana, Datadog, modern analytics. Instead of showing raw numbers, guide the operator through "what happened, why, what to do."

| Aspect | Flux Current | Data Storytelling |
|--------|-------------|-------------------|
| Metrics | Raw counts (`StatCard: 12`) | Contextual: "12 completed (+3 vs yesterday)" |
| Trends | None visible | Sparklines in every stat card |
| Failures | List of failed tasks | "3 failures today, all in `flux` project — build errors" |
| Insights | Separate page (planned Phase 3) | Inline summaries: "Opus 93% success vs Sonnet 87%" |

**What makes it work**: Operator can answer "how is the system doing?" in 3 seconds without reading individual cards. Actionable instead of merely informational. Dashboards transform from "data display" to "decision support."

**Implementation levels**:
1. **Minimal** (now): Add delta indicators to `StatCard` — "12 (+3)" with up/down arrow
2. **Medium** (Phase 3): Add sparklines via `recharts` `<Sparkline>` in stat cards
3. **Full** (Phase 3+): Narrative summaries — "Today: 12 tasks completed, 1 failure (build error in flux). All agents healthy."

**Tradeoffs**: Requires historical data (Phase 3 insights). Risk of over-interpreting small sample sizes early on.

**Verdict**: Adopt incrementally. Start with delta indicators in StatCard immediately. Sparklines and narrative summaries come with Phase 3 insights data.

---

### 5. Calm Technology — "Ambient awareness"

Inspired by Mark Weiser's calm computing. The system communicates status through ambient signals rather than demanding attention. Used in home automation UIs, Obsidian, Things 3.

| Aspect | Flux Current | Calm Technology |
|--------|-------------|----------------|
| Status | WS dot (green/red) in sidebar | Ambient color shift: page tint reflects system health |
| Notifications | Discord (external) | Subtle in-app toast that fades, not modal |
| Activity | Poll pods every 10s | Gentle pulse animation on active elements |
| Errors | Red badge/text | Soft amber glow that intensifies with severity |

**What makes it work**: Matches Flux's philosophy perfectly — operator sets direction, system runs autonomously. The UI should reassure, not demand attention. The dashboard becomes a "glanceable health check."

**Design principle**: If everything is healthy, the page looks serene. If something fails, a warm-toned accent appears without jarring the layout. The operator's attention is pulled proportionally to severity.

**Tradeoffs**: Can be too subtle. Critical failures (rate limit, crash recovery) need to be loud — Discord handles this already.

**Verdict**: Double down on existing calm aesthetic. The light theme + subtle shadows already lean calm. Enhance by making Dashboard a "health at a glance" view — serene when healthy, amber/rose accents surface proportionally when issues arise.

---

## Chosen Direction

Based on Flux's single-operator, autonomous-system context:

### Adopt

| Philosophy | Where | Priority | Effort |
|-----------|-------|----------|--------|
| **Command palette** (`Cmd+K`) | Global component | Highest | Low — single `CommandPalette.tsx` + `cmdk` |
| **Bento grid dashboard** | Dashboard page only | High | Medium — redesign Dashboard layout |
| **Data storytelling (deltas)** | StatCard component | High | Low — add delta prop + arrow indicator |
| **Calm technology** | Dashboard health signals | Medium | Low — ambient color based on system state |

### Keep

| Current Feature | Rationale |
|----------------|-----------|
| Sage Green & Light Blue dual point colors | More distinctive than yet another dark Linear clone |
| Semantic color tokens | Foundation for everything else |
| Compact information density | Operator needs data, not whitespace |
| Hand-built Tailwind components | No dependency bloat |
| Frosted collapsible sidebar | Already distinctive and functional |

### Skip

| Philosophy | Rationale |
|-----------|-----------|
| Full Linear dark rebrand | Homogeneous, doubles token maintenance, current light mode is more distinctive |
| AI-powered personalization | Single operator — no need for per-user adaptation |
| Chatbot-first interface | Flux operator interacts via Goals/Tasks, not chat |
| Zero-interface / proactive UI | Discord already handles proactive notifications |

---

## Implementation Roadmap

### Immediate (Can do now)

**1. Command Palette**
```
frontend/src/components/CommandPalette.tsx    (new)
frontend/src/App.tsx                         (add Cmd+K listener)
```
- Install `cmdk` (~3KB)
- Search across: tasks (by title), projects (by name), goals, navigation pages
- Actions: "Create task", "Go to dashboard", "Go to settings"
- Keyboard shortcut hints in results

**2. StatCard Deltas**
```
frontend/src/components/StatCard.tsx          (modify — add delta prop)
frontend/src/pages/Dashboard.tsx             (modify — compute deltas)
```
- Add optional `delta` and `deltaLabel` props to StatCard
- Show "12 (+3)" with green up-arrow or "5 (-2)" with red down-arrow
- Requires yesterday's counts (simple DB query or cached in insights)

### Phase 3 (With insights data)

**3. Bento Grid Dashboard**
```
frontend/src/pages/Dashboard.tsx             (rewrite layout)
```
- Hero tile: Active Goal (large, spans 2 columns)
- Medium tiles: Task pipeline (running/completed/failed with sparklines)
- Small tiles: Agent status, PR count, usage cost
- CSS Grid: `grid-template-columns: repeat(auto-fit, minmax(200px, 1fr))`

**4. Sparklines in StatCards**
```
frontend/src/components/StatCard.tsx          (modify — add sparkline)
```
- Use `recharts` `<ResponsiveContainer>` + `<AreaChart>` (mini, no axes)
- 7-day trend line behind the number
- Requires time-series data from Phase 3 insights collector

**5. Calm Health Signals**
```
frontend/src/pages/Dashboard.tsx             (modify — ambient state)
```
- Dashboard background subtly shifts based on system health:
  - All healthy: default serene page color
  - Warnings (rate limit, high failure rate): warm amber tint on hero tile border
  - Critical (agents down, crash recovery): rose accent on status indicators
- No modals or pop-ups — just color temperature shift

---

## Design Tokens Reference

### Point Colors — Sage Green & Light Blue

The two point colors define Flux's visual identity. Each is a full 7-step scale (50–700) that drives all primary UI elements. The operator selects one via the sidebar dot selector.

```
Light Blue (default)          Sage Green
─────────────────────         ─────────────────────
p50  #EFF4FF  hover tint      p50  #EFFDF4  hover tint
p100 #DBE6FE  badge bg        p100 #D4F7E2  badge bg
p200 #BFCFFD  focus ring      p200 #ABF0C8  focus ring
p300 #93B1FC  gradient end    p300 #73E2A3  gradient end
p400 #6B8CF8  hover state     p400 #3ECF80  hover state
p500 #4B6EF5  ■ PRIMARY       p500 #1DB960  ■ PRIMARY
p600 #3654E3  button bg       p600 #12964D  button bg
p700 #2C43C9  text accent     p700 #0F783F  text accent
```

### Semantic Surface Map

```
Page background     → var(--c-page)          Light Blue: #F7F9FE  │  Sage Green: #F5FAF6
Card surface        → var(--c-surface)       Both: #FFFFFF
Card hover          → var(--c-surface-hover) Light Blue: #F5F7FC  │  Sage Green: #F2F7F3
Card active/pressed → var(--c-surface-active)Light Blue: #EDF0F8  │  Sage Green: #E8F0EA
Alternate surface   → var(--c-surface-alt)   Light Blue: #F0F4FB  │  Sage Green: #EDF5EF
Deep surface        → var(--c-surface-deep)  Light Blue: #E8ECF5  │  Sage Green: #DFE9E1
Sidebar background  → var(--c-sidebar)       Light Blue: #F0F4FB  │  Sage Green: #EDF5EF
```

### Text & Border Map

```
Primary text        → var(--c-text)          Light Blue: #1A1F36  │  Sage Green: #1A2E1D
Secondary text      → var(--c-text-secondary)Light Blue: #5B6382  │  Sage Green: #506654
Muted text          → var(--c-text-muted)    Light Blue: #8B93AB  │  Sage Green: #7F9683
Faint text          → var(--c-text-faint)    Light Blue: #B5BBCE  │  Sage Green: #ACCAB0

Border              → var(--c-border)        Light Blue: #E2E6F0  │  Sage Green: #D4E2D6
Border hover        → var(--c-border-hover)  Light Blue: #CDD3E2  │  Sage Green: #B8CEB9
Border subtle       → var(--c-border-subtle) Light Blue: #EDF0F7  │  Sage Green: #E5F0E7
```

### Status Colors (Theme-Independent)

```
Success   → emerald-600  #059669   Task completed, PR merged, tests passed
Danger    → rose-600     #E11D48   Task failed, critical alert, cancel
Warning   → amber-600    #D97706   Rate limit, degraded state
Info      → cyan-600     #0891B2   Running state, informational
Purple    → violet-600   #7C3AED   Decomposed tasks, special states
```

### Typography Scale

| Token | Size | Weight | Use |
|-------|------|--------|-----|
| Micro label | 10-11px | 500 | Category headers, filter labels |
| Body small | 13px | 400-500 | Nav items, card content, table cells |
| Body | 14px (text-sm) | 400 | Default text, descriptions |
| Heading | 16-20px | 600-700 | Page titles, section headers |
| Metric | 24px+ | 700 | StatCard numbers (tabular-nums) |
| Mono | 13px | 400 | Code, JSON, log output |

### Spacing System

| Token | Value | Use |
|-------|-------|-----|
| Card padding | `p-4` to `p-5` | Interior card spacing |
| Page padding | `p-5 sm:p-6 lg:p-8` | Responsive page margins |
| Card gap | `gap-4` | Between cards in grid |
| Section gap | `space-y-6` | Between page sections |
| Nav item gap | `space-y-0.5` | Between sidebar items |
| Touch target | `min-h-[44px]` | All interactive elements |

---

## References

- [Linear Design: The SaaS Trend](https://blog.logrocket.com/ux-design/linear-design/)
- [The Rise of Linear Style Design](https://medium.com/design-bootcamp/the-rise-of-linear-style-design-origins-trends-and-techniques-4fd96aab7646)
- [How We Redesigned the Linear UI](https://linear.app/now/how-we-redesigned-the-linear-ui)
- [Bento Grid Dashboard Design](https://www.orbix.studio/blogs/bento-grid-dashboard-design-aesthetics)
- [Bento Grids for AI Dashboards](https://baltech.in/blog/bento-grids-for-ai-dashboards/)
- [Top Dashboard Design Trends 2025](https://fuselabcreative.com/top-dashboard-design-trends-2025/)
- [Design Engineering at Vercel](https://vercel.com/blog/design-engineering-at-vercel)
- [Building Dark Mode at Sentry](https://blog.sentry.io/building-dark-mode/)
- [12 UI/UX Design Trends 2026](https://www.index.dev/blog/ui-ux-design-trends)
- [8 UI Design Trends 2025](https://www.pixelmatters.com/insights/8-ui-design-trends-2025)
