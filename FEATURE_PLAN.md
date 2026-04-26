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

## 3. GUI Mode Themed Off Windows 98
- **Purpose:**
  - Provide a graphical user interface inspired by Windows 98.
- **Requirements:**
  - Use [98.css](https://jdan.github.io/98.css/) for styling.
  - Integrate with Wails for desktop app functionality.
  - Replicate classic Win98 UI elements (windows, buttons, menus).
- **Implementation Notes:**
  - Ensure accessibility and usability.
  - Allow switching between CLI and GUI modes.

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
