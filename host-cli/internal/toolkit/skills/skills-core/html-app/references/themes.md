# Themes

Adapted from the themes of Anthropic's theme-factory skill (Apache License
2.0, https://github.com/anthropics/skills), with the fonts mapped to the two
faces this drive ships: Inter (sans) and Fraunces (serif). Paste a theme's
lines into the template's `EDIT: theme` region.

Each theme gives `--app-background`, `--app-text`, `--app-accent`,
`--app-accent-text`, `--app-soft` and the two fonts. Check that text on
the accent colour stays readable; swap `--app-accent-text` to the dark
colour where it does not.

## Ocean Depths (calm, professional)

```css
--app-background: #f1faee; --app-text: #1a2332; --app-accent: #2d8b8b; --app-accent-text: #ffffff; --app-soft: #a8dadc;
--app-heading-font: "Inter", system-ui, sans-serif; --app-body-font: "Inter", system-ui, sans-serif;
```

## Sunset Boulevard (warm, lively)

```css
--app-background: #fdf6ee; --app-text: #264653; --app-accent: #e76f51; --app-accent-text: #ffffff; --app-soft: #f4a261;
--app-heading-font: "Fraunces", Georgia, serif; --app-body-font: "Inter", system-ui, sans-serif;
```

## Forest Canopy (natural, grounded)

```css
--app-background: #faf9f6; --app-text: #2d4a2b; --app-accent: #7d8471; --app-accent-text: #ffffff; --app-soft: #a4ac86;
--app-heading-font: "Fraunces", Georgia, serif; --app-body-font: "Inter", system-ui, sans-serif;
```

## Modern Minimalist (clean, grey)

```css
--app-background: #ffffff; --app-text: #36454f; --app-accent: #708090; --app-accent-text: #ffffff; --app-soft: #d3d3d3;
--app-heading-font: "Inter", system-ui, sans-serif; --app-body-font: "Inter", system-ui, sans-serif;
```

## Golden Hour (rich, autumnal)

```css
--app-background: #fffaf0; --app-text: #4a403a; --app-accent: #c1666b; --app-accent-text: #ffffff; --app-soft: #f4a900;
--app-heading-font: "Fraunces", Georgia, serif; --app-body-font: "Inter", system-ui, sans-serif;
```

## Arctic Frost (cool, crisp)

```css
--app-background: #fafafa; --app-text: #22324a; --app-accent: #4a6fa5; --app-accent-text: #ffffff; --app-soft: #d4e4f7;
--app-heading-font: "Inter", system-ui, sans-serif; --app-body-font: "Inter", system-ui, sans-serif;
```

## Desert Rose (soft, refined)

```css
--app-background: #f7efe8; --app-text: #5d2e46; --app-accent: #b87d6d; --app-accent-text: #ffffff; --app-soft: #d4a5a5;
--app-heading-font: "Inter", system-ui, sans-serif; --app-body-font: "Inter", system-ui, sans-serif;
```

## Tech Innovation (bold, dark)

```css
--app-background: #1e1e1e; --app-text: #f5f5f5; --app-accent: #0066ff; --app-accent-text: #ffffff; --app-soft: #00ffff;
--app-heading-font: "Inter", system-ui, sans-serif; --app-body-font: "Inter", system-ui, sans-serif;
```

With a dark background also set `data-theme="dark"` on the `<html>` tag.

## Botanical Garden (fresh, organic)

```css
--app-background: #f5f3ed; --app-text: #2f3e2c; --app-accent: #4a7c59; --app-accent-text: #ffffff; --app-soft: #f9a620;
--app-heading-font: "Fraunces", Georgia, serif; --app-body-font: "Inter", system-ui, sans-serif;
```

## Midnight Galaxy (dramatic, deep)

```css
--app-background: #2b1e3e; --app-text: #e6e6fa; --app-accent: #a490c2; --app-accent-text: #2b1e3e; --app-soft: #4a4e8f;
--app-heading-font: "Fraunces", Georgia, serif; --app-body-font: "Inter", system-ui, sans-serif;
```

With a dark background also set `data-theme="dark"` on the `<html>` tag.

## Making your own

Pick a background and a text colour with strong contrast, one accent for
buttons and links, and one soft colour for borders. Use Fraunces for
headings when the page should feel editorial, Inter alone when it should
feel like a tool.
