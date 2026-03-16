# Sort Dashboards by Updated Date Using Version API

## Overview

Enhance the dashboard listing to fetch the actual last-update timestamp from the Grafana dashboard versions API. For each dashboard, extract the `version` field from the dashboard JSON, call `/api/dashboards/uid/:uid/versions/:version` to get the `created` timestamp of that version, and use it as the dashboard's updated date. Also display the dashboard version number next to the dashboard title in the UI.

## Context

- Files involved: `main.go`, `public/js/app.js`
- Related patterns: `fetchDashboardDetails` already fetches per-dashboard details concurrently with a semaphore
- Grafana API: `/api/dashboards/uid/:uid/versions/:version` returns version info including `created` timestamp

## Development Approach

- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Add Version field to Dashboard struct and fetch version details in backend

**Files:**
- Modify: `main.go`

- [x] Add `Version int` field to the `Dashboard` struct with JSON tag `"version"`
- [x] In `fetchDashboardDetails`, after fetching `/api/dashboards/uid/:uid`, extract the `version` number from `dashboardDetail.Dashboard["version"]`
- [x] Set `dash.Version` from the extracted version number
- [x] Call `/api/dashboards/uid/:uid/versions/:version` using the extracted version number
- [x] Parse the response and extract the `created` field as the dashboard's `Updated` timestamp
- [x] If the versions API call fails, fall back to using the existing `updated` field from the dashboard detail
- [x] Write tests for the version extraction and timestamp logic
- [x] Run project test suite - must pass before task 2

### Task 2: Display version number next to dashboard title in the frontend

**Files:**
- Modify: `public/js/app.js`

- [x] In `renderDashboards`, append the version number to the dashboard title display (e.g., "Dashboard Name (v3)")
- [x] Only show version if the value is present and greater than 0
- [x] Verify sorting by "recently-updated" and "oldest-updated" still works correctly with the new timestamp source
- [x] Write/update tests if applicable
- [x] Run project test suite - must pass before task 3

### Task 3: Verify acceptance criteria

- [x] Manual test: load the dashboard list and verify version numbers appear next to titles
- [x] Manual test: sort by "recently-updated" and confirm dashboards are ordered by their actual last version creation date
- [x] Run full test suite (`go test -v ./...`)
- [x] Verify test coverage meets 80%+

### Task 4: Update documentation

- [ ] Update CLAUDE.md if internal patterns changed
- [ ] Move this plan to `docs/plans/completed/`
