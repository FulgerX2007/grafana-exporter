// DOM Elements
const foldersList = document.getElementById('foldersList');
const dashboardsContainer = document.getElementById('dashboardsContainer');
const searchDashboard = document.getElementById('searchDashboard');
const selectAllBtn = document.getElementById('selectAllBtn');
const clearSelectionBtn = document.getElementById('clearSelectionBtn');
const includeLibrariesCheck = document.getElementById('includeLibrariesCheck');
const selectedCountElement = document.getElementById('selectedCount');
const exportBtn = document.getElementById('exportBtn');
const loadingOverlay = document.getElementById('loadingOverlay');
const loadingText = document.getElementById('loadingText');
const debugInfo = document.getElementById('debugInfo');
const exportResultSection = document.getElementById('exportResultSection');
const exportResultContent = document.getElementById('exportResultContent');
const alertContainer = document.getElementById('alertContainer');

// State
let folders = [];
let dashboards = [];
let filteredDashboards = [];
let selectedDashboards = new Set();
let selectedFolder = 'all';
let searchQuery = '';
let lastFolderClick = Date.now();

// Initialize the application
document.addEventListener('DOMContentLoaded', initialize);

function initialize() {
    // Setup event listeners
    searchDashboard.addEventListener('input', handleSearchInput);
    selectAllBtn.addEventListener('click', selectAllDashboards);
    clearSelectionBtn.addEventListener('click', clearDashboardSelection);
    exportBtn.addEventListener('click', exportSelectedDashboards);

    // Load data
    loadFolders();
    loadDashboards();
}

// API Functions
async function loadDashboards() {
    try {
        showLoading('Loading dashboards...', 'Fetching from API...');

        const response = await fetch('/api/dashboards');
        if (!response.ok) throw new Error(`Failed to load dashboards: ${response.statusText}`);

        showLoading('Loading dashboards...', 'Processing dashboard data...');
        const data = await response.json();
        dashboards = data.dashboards || [];

        // Add a folderTitle property for General folder
        dashboards.forEach(dashboard => {
            if (dashboard.folderId === 0 && !dashboard.folderTitle) {
                dashboard.folderTitle = "General";
            }
        });

        console.log("Loaded dashboards:", dashboards.length);
        if (dashboards.length > 0) {
            console.log("Sample dashboard:", dashboards[0]);
        } else {
            console.log("No dashboards found");
        }

        filteredDashboards = [...dashboards];
        showLoading('Loading dashboards...', `Found ${dashboards.length} dashboards. Rendering...`);

        // Set a small timeout to ensure the UI updates
        setTimeout(() => {
            hideLoading();
            renderDashboards();
        }, 500);
    } catch (error) {
        hideLoading();
        showAlert('error', `Error loading dashboards: ${error.message}`);
    }
}

async function exportSelectedDashboards() {
    if (selectedDashboards.size === 0) {
        showAlert('warning', 'Please select at least one dashboard to export');
        return;
    }

    try {
        showLoading('Exporting dashboards and linked libraries...');

        const dashboardUIDs = Array.from(selectedDashboards);

        const response = await fetch('/api/export', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ dashboardUIDs })
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`Export failed: ${errorText}`);
        }

        const result = await response.json();
        hideLoading();

        // Show results
        showExportResults(result);
    } catch (error) {
        hideLoading();
        showAlert('error', `Export failed: ${error.message}`);
    }
}

// UI Rendering Functions
function renderFolders() {
    // Always include "All Folders" option
    let html = `<div class="list-group-item folder-item ${selectedFolder === 'all' ? 'selected-folder' : ''}"
                    data-folder-id="all">All Folders</div>`;

    // Include "General" folder (root)
    html += `<div class="list-group-item folder-item ${selectedFolder === '0' ? 'selected-folder' : ''}"
                data-folder-id="0">General</div>`;

    // Create a map of parent-child relationships
    const folderMap = new Map();
    const rootFolders = [];

    // First pass: identify all folders and their parents
    folders.forEach(folder => {
        // If it has a parentUid, it's a nested folder
        if (folder.parentUid) {
            // Get or create array of children for this parent
            if (!folderMap.has(folder.parentUid)) {
                folderMap.set(folder.parentUid, []);
            }
            folderMap.get(folder.parentUid).push(folder);
        } else {
            // It's a root folder
            rootFolders.push(folder);
        }
    });

    // Function to recursively render folders with proper indentation
    function renderFolder(folder, level) {
        const indent = level * 20; // 20px indent per level
        const count = folder.dashboardCount || 0;
        const countBadge = count > 0 ? `<span class="badge bg-secondary ms-2">${count}</span>` : '';

        // Add level indicator badge
        const levelBadge = `<span class="badge bg-primary me-2">L${level}</span>`;

        html += `<div class="list-group-item folder-item ${selectedFolder === folder.id.toString() ? 'selected-folder' : ''}"
                    data-folder-id="${folder.id}" style="padding-left: ${indent + 15}px;">
                    <div class="d-flex justify-content-between align-items-center">
                        <span>${levelBadge}${folder.title}</span>
                        ${countBadge}
                    </div>
                </div>`;

        // Render children if any
        if (folderMap.has(folder.uid)) {
            folderMap.get(folder.uid).forEach(child => {
                renderFolder(child, level + 1);
            });
        }
    }

    // Render all root folders and their children
    rootFolders.forEach(folder => {
        renderFolder(folder, 1); // Start at level 1 for root folders
    });

    foldersList.innerHTML = html;

    // Add event listeners
    document.querySelectorAll('.folder-item').forEach(item => {
        item.addEventListener('click', (e) => {
            // Debounce clicks to prevent multiple rapid clicks
            const now = Date.now();
            if (now - lastFolderClick < 300) {
                return;
            }
            lastFolderClick = now;

            const folderId = e.currentTarget.dataset.folderId;
            selectedFolder = folderId;

            // Update UI
            document.querySelectorAll('.folder-item').forEach(el => {
                el.classList.remove('selected-folder');
            });
            e.currentTarget.classList.add('selected-folder');

            // Show loading while filtering
            showLoading('Filtering dashboards...', `Filtering by folder ID: ${folderId}`);

            // Use setTimeout to allow the UI to update
            setTimeout(() => {
                filterDashboards();
                hideLoading();
            }, 100);
        });
    });
}

function renderDashboards() {
    if (filteredDashboards.length === 0) {
        dashboardsContainer.innerHTML = `
            <div class="text-center py-5 text-muted">
                <p>No dashboards found matching your criteria</p>
            </div>
        `;
        return;
    }

    let html = '';
    filteredDashboards.forEach(dashboard => {
        const isSelected = selectedDashboards.has(dashboard.uid);
        const folderName = dashboard.folderTitle || 'General';

        html += `
            <div class="dashboard-item ${isSelected ? 'selected' : ''}" data-uid="${dashboard.uid}">
                <div class="d-flex justify-content-between align-items-center">
                    <div>
                        <div class="fw-medium">${dashboard.title}</div>
                        <div class="small text-muted">Folder: ${folderName}</div>
                    </div>
                    <div class="form-check">
                        <input class="form-check-input dashboard-checkbox" type="checkbox"
                            id="check_${dashboard.uid}" data-uid="${dashboard.uid}" ${isSelected ? 'checked' : ''}>
                    </div>
                </div>
            </div>
        `;
    });

    dashboardsContainer.innerHTML = html;

    // Add event listeners
    document.querySelectorAll('.dashboard-item').forEach(item => {
        item.addEventListener('click', function(e) {
            if (e.target.classList.contains('form-check-input')) return;

            const uid = this.dataset.uid;
            const checkbox = document.getElementById(`check_${uid}`);

            checkbox.checked = !checkbox.checked;
            toggleDashboardSelection(uid, checkbox.checked);
        });
    });

    document.querySelectorAll('.dashboard-checkbox').forEach(checkbox => {
        checkbox.addEventListener('change', function() {
            const uid = this.dataset.uid;
            toggleDashboardSelection(uid, this.checked);
        });
    });

    updateSelectedCount();
}

function filterDashboards() {
    console.log("Filtering dashboards...");
    console.log("Selected folder:", selectedFolder);
    console.log("Total dashboards:", dashboards.length);

    // First, filter by folder
    if (selectedFolder === 'all') {
        // All folders - no filtering needed
        filteredDashboards = [...dashboards];
    } else {
        const folderId = parseInt(selectedFolder);
        filteredDashboards = dashboards.filter(dashboard => {
            return dashboard.folderId === folderId;
        });
    }

    // Then, filter by search term
    if (searchQuery && searchQuery.length > 0) {
        const query = searchQuery.toLowerCase();
        filteredDashboards = filteredDashboards.filter(dashboard => {
            return dashboard.title.toLowerCase().includes(query);
        });
    }

    console.log("Filtered dashboards:", filteredDashboards.length);
    renderDashboards();
}

function handleSearchInput() {
    searchQuery = searchDashboard.value.trim();
    filterDashboards();
}

function toggleDashboardSelection(uid, isSelected) {
    if (isSelected) {
        selectedDashboards.add(uid);
    } else {
        selectedDashboards.delete(uid);
    }

    // Update UI
    const dashboardItem = document.querySelector(`.dashboard-item[data-uid="${uid}"]`);
    if (dashboardItem) {
        if (isSelected) {
            dashboardItem.classList.add('selected');
        } else {
            dashboardItem.classList.remove('selected');
        }
    }

    updateSelectedCount();
}

function selectAllDashboards() {
    filteredDashboards.forEach(dashboard => {
        selectedDashboards.add(dashboard.uid);
        const checkbox = document.getElementById(`check_${dashboard.uid}`);
        if (checkbox) checkbox.checked = true;
    });

    renderDashboards();
}

function clearDashboardSelection() {
    selectedDashboards.clear();
    document.querySelectorAll('.dashboard-checkbox').forEach(checkbox => {
        checkbox.checked = false;
    });

    renderDashboards();
}

function updateSelectedCount() {
    const count = selectedDashboards.size;
    selectedCountElement.textContent = count;
    exportBtn.disabled = count === 0;
}

function showExportResults(result) {
    exportResultSection.style.display = 'block';

    let html = `
        <div class="alert alert-success">
            <h5>Export Completed</h5>
            <p>Successfully exported ${result.exportedDashboards} dashboards and ${result.exportedLibraries} linked library panels.</p>
            <p>Export path: <code>${result.exportPath}</code></p>
        </div>
    `;

    if (result.errors && result.errors.length > 0) {
        html += `
            <div class="alert alert-warning mt-3">
                <h5>Warnings/Errors</h5>
                <ul>
                    ${result.errors.map(error => `<li>${error}</li>`).join('')}
                </ul>
            </div>
        `;
    }

    exportResultContent.innerHTML = html;

    // Scroll to results
    exportResultSection.scrollIntoView({ behavior: 'smooth' });
}

// Utility Functions
function showLoading(message, debug = '') {
    loadingText.textContent = message || 'Processing...';
    debugInfo.textContent = debug;
    loadingOverlay.style.display = 'flex';
}

function hideLoading() {
    loadingOverlay.style.display = 'none';
}

function showAlert(type, message) {
    const alertId = 'alert_' + Date.now();
    const alertHtml = `
        <div id="${alertId}" class="alert alert-${type} alert-dismissible fade show" role="alert">
            ${message}
            <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
        </div>
    `;

    alertContainer.insertAdjacentHTML('beforeend', alertHtml);

    // Auto-dismiss after 5 seconds
    setTimeout(() => {
        const alertElement = document.getElementById(alertId);
        if (alertElement) {
            const bsAlert = new bootstrap.Alert(alertElement);
            bsAlert.close();
        }
    }, 5000);
}

async function loadFolders() {
    try {
        const response = await fetch('/api/folders');
        if (!response.ok) throw new Error(`Failed to load folders: ${response.statusText}`);

        const data = await response.json();

        // Check if we got the enhanced debug response
        if (data.folders) {
            folders = data.folders;
            console.log("Loaded folders:", folders.length);
            console.log("Has nested structure:", data.hasNestedStructure);
            console.log("Debug info:", data.debug);

            // Display debug info
            showAlert(data.hasNestedStructure ? 'info' : 'warning',
                      `Folder info: ${data.debug}`);
        } else {
            // Backward compatibility
            folders = data;
            console.log("Loaded folders:", folders.length);
        }

        // Check if any folders have parentUid
        const nestedFolders = folders.filter(f => f.parentUid);
        console.log("Folders with parentUid:", nestedFolders.length);
        if (nestedFolders.length > 0) {
            console.log("Example nested folder:", nestedFolders[0]);
        }

        renderFolders();
    } catch (error) {
        showAlert('error', `Error loading folders: ${error.message}`);
    }
}