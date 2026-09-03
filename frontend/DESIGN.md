---
name: Obsidian Deep
colors:
  surface: '#151219'
  surface-dim: '#151219'
  surface-bright: '#3b383f'
  surface-container-lowest: '#0f0d14'
  surface-container-low: '#1d1a21'
  surface-container: '#211e25'
  surface-container-high: '#2c2930'
  surface-container-highest: '#36333b'
  on-surface: '#e7e0ea'
  on-surface-variant: '#cbc3d4'
  inverse-surface: '#e7e0ea'
  inverse-on-surface: '#322f37'
  outline: '#958e9d'
  outline-variant: '#494552'
  surface-tint: '#d1bcff'
  primary: '#d1bcff'
  on-primary: '#3c1084'
  primary-container: '#9f7dec'
  on-primary-container: '#35027e'
  inverse-primary: '#6b49b5'
  secondary: '#c6c6c7'
  on-secondary: '#2f3131'
  secondary-container: '#454747'
  on-secondary-container: '#b4b5b5'
  tertiary: '#e2c545'
  on-tertiary: '#3a3000'
  tertiary-container: '#c5aa2a'
  on-tertiary-container: '#4b3f00'
  error: '#ffb4ab'
  on-error: '#690005'
  error-container: '#93000a'
  on-error-container: '#ffdad6'
  primary-fixed: '#eaddff'
  primary-fixed-dim: '#d1bcff'
  on-primary-fixed: '#24005b'
  on-primary-fixed-variant: '#532f9b'
  secondary-fixed: '#e2e2e2'
  secondary-fixed-dim: '#c6c6c7'
  on-secondary-fixed: '#1a1c1c'
  on-secondary-fixed-variant: '#454747'
  tertiary-fixed: '#ffe263'
  tertiary-fixed-dim: '#e2c545'
  on-tertiary-fixed: '#221b00'
  on-tertiary-fixed-variant: '#534600'
  background: '#151219'
  on-background: '#e7e0ea'
  surface-variant: '#36333b'
typography:
  h1:
    fontFamily: manrope
    fontSize: 2.5rem
    fontWeight: '700'
    lineHeight: '1.2'
    letterSpacing: -0.02em
  h2:
    fontFamily: manrope
    fontSize: 1.875rem
    fontWeight: '600'
    lineHeight: '1.3'
  h3:
    fontFamily: manrope
    fontSize: 1.5rem
    fontWeight: '600'
    lineHeight: '1.4'
  body-lg:
    fontFamily: inter
    fontSize: 1.125rem
    fontWeight: '400'
    lineHeight: '1.6'
  body-md:
    fontFamily: inter
    fontSize: 1rem
    fontWeight: '400'
    lineHeight: '1.6'
  body-sm:
    fontFamily: inter
    fontSize: 0.875rem
    fontWeight: '400'
    lineHeight: '1.5'
  label-md:
    fontFamily: inter
    fontSize: 0.875rem
    fontWeight: '500'
    lineHeight: '1'
    letterSpacing: 0.01em
  code:
    fontFamily: spaceGrotesk
    fontSize: 0.9rem
    fontWeight: '400'
    lineHeight: '1.5'
rounded:
  sm: 0.125rem
  DEFAULT: 0.25rem
  md: 0.375rem
  lg: 0.5rem
  xl: 0.75rem
  full: 9999px
spacing:
  base: 4px
  xs: 0.25rem
  sm: 0.5rem
  md: 1rem
  lg: 1.5rem
  xl: 2.5rem
  gutter: 1rem
  max-width: 800px
---

## Brand & Style

This design system is built for the high-performance knowledge worker. It centers on the concept of "Digital Zen"—a state of deep work where the interface disappears, leaving only the user's thoughts and connections. The aesthetic is a rigorous interpretation of **Minimalism**, prioritizing content hierarchy and information density over decorative elements. 

The target audience consists of researchers, developers, and writers who require a high-readability environment that reduces eye strain during long sessions. The brand personality is intellectual, precise, and utilitarian, evoking an emotional response of focus, calm, and structural order.

## Colors

The palette is rooted in a deep, neutral charcoal-black base to provide a void-like canvas for thought. The primary accent is a **soft purple**, used sparingly for high-value interactions and active states, providing a sophisticated alternative to traditional tech blues. 

Surface colors move through a tight grayscale spectrum to create structural separation without relying on heavy borders or shadows. Neutral whites and grays are reserved for typography and secondary UI icons to ensure maximum legibility against the dark backdrop.

## Typography

This design system utilizes a dual-font strategy. **Manrope** is used for headlines to provide a modern, refined, and slightly more geometric character to the structure of the document. **Inter** serves as the primary body and UI font, selected for its exceptional legibility and systematic, utilitarian feel at all sizes. 

For technical snippets or metadata, a monospaced-adjacent variant like **Space Grotesk** can be used to distinguish "data" from "prose." Paragraphs maintain a generous line height (1.6) to prevent visual fatigue during long-form reading.

## Layout & Spacing

The layout follows a **Fixed Grid** philosophy for the central reading experience, constraining the main content column to 800px to ensure optimal line length. Side panels (file explorers, inspectors, and backlinks) utilize a fluid model, pinning to the left and right edges of the viewport.

Spacing follows a strict 4px baseline grid. Internal component padding should be compact (8px-12px) to maintain high information density, while the vertical rhythm between major sections of text remains spacious (24px-40px) to provide visual breathing room.

## Elevation & Depth

Depth is conveyed through **Tonal Layers** rather than shadows. In a dark environment, light shadows look muddy; instead, we lift elements by lightening their surface color.

1.  **Background (#171717):** The bottom-most layer, used for the main workspace.
2.  **Surface Low (#1E1E1E):** Sidebars and navigation panels.
3.  **Surface Medium (#262626):** Active states, tooltips, and floating modals.
4.  **Surface High (#333333):** Hover states on buttons or list items.

Low-contrast outlines (#2E2E2E) are used to define boundaries between panels of the same tonal level, maintaining a flat, architectural feel.

## Shapes

The shape language is "Soft" (0.25rem), leaning toward a more serious and professional appearance. Sharp corners are avoided to keep the UI from feeling aggressive, but large curves are rejected to maintain the "knowledge base" aesthetic. 

Small UI elements like checkboxes, chips, and small buttons use the base `0.25rem` radius. Larger container elements like cards or modals may scale to `0.5rem` (`rounded-lg`), but never more, ensuring the interface feels structural and grounded.

## Components

-   **Buttons:** Primary buttons use the soft purple background with dark text for maximum contrast. Secondary buttons use a subtle gray outline or a "ghost" style with no background until hovered.
-   **Chips/Tags:** Small, low-contrast pills using `Surface High` background and `Text Secondary`. On hover, they transition to the primary purple text.
-   **Lists:** High-density list items with 4px vertical padding. Active items are indicated by a 2px vertical purple line on the left edge.
-   **Input Fields:** Minimalist design with a 1px border (`Border Subtle`). Focus states swap the border for the primary purple without adding glow.
-   **Cards:** Non-shadowed containers using `Surface Low`. Hierarchy is established through typography and internal spacing rather than external elevation.
-   **Knowledge Graph Nodes:** Circular nodes using the primary accent color for active notes and neutral gray for unlinked notes, connected by low-opacity gray lines.