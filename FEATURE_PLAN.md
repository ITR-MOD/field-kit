# Feature Plan for itr-field-kit

## Overview
This document outlines the planned features for the itr-field-kit project. Each feature is described with its purpose, requirements, and implementation notes.

---

## 1. Optional Metadata File
- **Purpose:**
  - Allow users to include an optional metadata file alongside their mod or package.
- **Requirements:**
  - Detect and parse a metadata file if present.
  - Define supported formats (e.g., JSON, YAML, TOML).
  - Expose metadata to other features (e.g., installer, GUI).
- **Implementation Notes:**
  - File name convention (e.g., `modinfo.json`, `metadata.yaml`).
  - Graceful fallback if file is missing.

---

## 2. FOMOD Installer Support
- **Purpose:**
  - Support FOMOD installer format for mod installation workflows.
- **Requirements:**
  - Parse FOMOD XML files.
  - Present install options to user (CLI and GUI).
  - Handle file copying and option selection logic.
- **Implementation Notes:**
  - Use existing FOMOD parsing libraries if available.
  - Ensure compatibility with common FOMOD structures.

---

## 3. GUI Mode — UNPSC Theme (ITR2-Inspired)
- **Purpose:**
  - Provide a graphical user interface themed after the UNPSC faction aesthetic from Into The Radius 2.
- **Requirements:**
  - Implement via Wails (Go backend + web frontend).
  - Follow the UNPSC UI design spec in `.references/UNPSC_UI_Design_Doc.md`.
  - Dark-first: `#1A1C1E` background, `#4ECDC4` teal primary accent, `#F4A72A` amber warning, `#C0392B` danger.
  - Fonts: Share Tech Mono (headers/data), Barlow Condensed (UI labels/nav), Roboto (body).
  - Squared corners (0–2px radius), hard 1px `#3D4347` borders — no rounded cards.
  - Dense, left-aligned, structured column layouts (military inventory form density).
  - CRT scan-line texture on terminal/header surfaces (CSS `repeating-linear-gradient`).
  - ALL CAPS nav labels, terse bureaucratic copy tone.
  - Allow switching between TUI and GUI modes via CLI flag.
- **Implementation Notes:**
  - Custom CSS — do NOT use 98.css or similar pre-built theme libraries.
  - Design token reference: `.references/UNPSC_UI_Design_Doc.md` §11.
  - TUI (ITR1 green phosphor) and GUI (ITR2 UNPSC steel/teal) are parallel modes, same Go backend.
  - Wails bridges Go `internal/` packages directly to the frontend.

---

## Next Steps
#
---

## 4. Load Order Configuration & Dynamic Filename Adjustment
- **Purpose:**
  - Allow users to specify a custom load order for mods/files, overriding the default alpha-numerical order.
- **Requirements:**
  - Provide a configuration method (e.g., in the profile or a separate config file) to define load order.
  - By default, files are loaded in alpha-numerical order (current behavior).
  - When a custom load order is set, redeployment should rename all deployed files to reflect the new order.
  - Use a numeric prefix (up to 4 digits) on filenames to enforce the order (e.g., `0001_file.ext`, `0002_file.ext`).
  - Changing the load order requires a full redeployment and renaming of all affected files.
- **Implementation Notes:**
  - Must handle renaming and cleanup of old files on redeploy.
  - Ensure no filename collisions or loss of data during renaming.
  - Integrate with the deployment logic so that the deployed state reflects the current load order.
  - Consider user experience for both CLI and GUI modes.

- Expand each feature with detailed tasks and milestones.
- Assign priorities and dependencies.
- Review and update as implementation progresses.
