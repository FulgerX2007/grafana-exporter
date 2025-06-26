const foldersList = document.getElementById('foldersList');
const dashboardsContainer = document.getElementById('dashboardsContainer');
const searchDashboard = document.getElementById('searchDashboard');
const selectAllBtn = document.getElementById('selectAllBtn');
const clearSelectionBtn = document.getElementById('clearSelectionBtn');
const includeLibrariesCheck = document.getElementById('includeLibrariesCheck');
const includeAlertsCheck = document.getElementById('includeAlertsCheck');
const selectedCountElement = document.getElementById('selectedCount');
const exportBtn = document.getElementById('exportBtn');
const loadingOverlay = document.getElementById('loadingOverlay');
const loadingText = document.getElementById('loadingText');
const debugInfo = document.getElementById('debugInfo');
const exportResultSection = document.getElementById('exportResultSection');
const exportResultContent = document.getElementById('exportResultContent');
const alertContainer = document.getElementById('alertContainer');
const alertsContainer = document.getElementById('alertsContainer');
const searchAlerts = document.getElementById('searchAlerts');
const selectAllAlertsBtn = document.getElementById('selectAllAlertsBtn');
const clearAlertsSelectionBtn = document.getElementById('clearAlertsSelectionBtn');
const exportAsZipCheck = document.getElementById('exportAsZipCheck');

let folders = [];
let dashboards = [];
let filteredDashboards = [];
let selectedDashboards = new Set();
let selectedFolder = 'all';
let searchQuery = '';
let lastFolderClick = Date.now();
let alerts = [];
let filteredAlerts = [];
let selectedAlerts = new Set();
let alertSearchQuery = '';
let selectedAlertFolder = 'all';
let appConfig = {
    forceEnableZipExport: false
};

document.addEventListener('DOMContentLoaded', initialize);

function initialize() {
    searchDashboard.addEventListener('input', handleSearchInput);
    selectAllBtn.addEventListener('click', selectAllDashboards);
    clearSelectionBtn.addEventListener('click', clearDashboardSelection);
    exportBtn.addEventListener('click', exportSelectedDashboards);
    
    searchAlerts.addEventListener('input', handleAlertSearchInput);
    selectAllAlertsBtn.addEventListener('click', selectAllAlerts);
    clearAlertsSelectionBtn.addEventListener('click', clearAlertSelection);

    loadConfig();
    loadFolders();
    loadDashboards();
    loadAlerts();
}

async function loadDashboards() {
    try {
        showLoading('Loading dashboards...', 'Fetching from API...');

        const response = await fetch('/api/dashboards');
        if (!response.ok) throw new Error(`Failed to load dashboards: ${response.statusText}`);

        showLoading('Loading dashboards...', 'Processing dashboard data...');
        const data = await response.json();
        dashboards = data.dashboards || [];

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

        setTimeout(() => {
            hideLoading();
            renderDashboards();
        }, 500);
    } catch (error) {
        hideLoading();
        showAlert('error', `Error loading dashboards: ${error.message}`);
    }
}

async function loadAlerts() {
    try {
        showLoading('Loading alerts...', 'Fetching from API...');

        const response = await fetch('/api/alerts');
        if (!response.ok) throw new Error(`Failed to load alerts: ${response.statusText}`);

        showLoading('Loading alerts...', 'Processing alert data...');
        const data = await response.json();
        alerts = data.alerts || [];

        console.log("Loaded alerts:", alerts.length);
        if (alerts.length > 0) {
            console.log("Sample alert:", alerts[0]);
        } else {
            console.log("No alerts found");
        }

        filteredAlerts = [...alerts];
        showLoading('Loading alerts...', `Found ${alerts.length} alerts. Rendering...`);

        setTimeout(() => {
            hideLoading();
            renderAlerts();
        }, 500);
    } catch (error) {
        hideLoading();
        showAlert('error', `Error loading alerts: ${error.message}`);
    }
}

function renderFolders() {
    renderDashboardFolders();
    renderAlertFolders();
}

function renderDashboardFolders() {
    let html = `<div class="list-group-item folder-item ${selectedFolder === 'all' ? 'selected-folder' : ''}"
                    data-folder-id="all">All Folders</div>`;

    html += `<div class="list-group-item folder-item ${selectedFolder === '0' ? 'selected-folder' : ''}"
                data-folder-id="0">General</div>`;

    const folderMap = new Map();
    const rootFolders = [];

    folders.forEach(folder => {
        if (folder.parentUid) {
            if (!folderMap.has(folder.parentUid)) {
                folderMap.set(folder.parentUid, []);
            }
            folderMap.get(folder.parentUid).push(folder);
        } else {
            rootFolders.push(folder);
        }
    });

    function renderFolder(folder, level) {
        const indent = level * 20;
        const count = folder.dashboardCount || 0;
        const countBadge = count > 0 ? `<span class="badge bg-secondary ms-2">${count}</span>` : '';

        const levelBadge = `<span class="badge bg-primary me-2">L${level}</span>`;

        html += `<div class="list-group-item folder-item ${selectedFolder === folder.id.toString() ? 'selected-folder' : ''}"
                    data-folder-id="${folder.id}" style="padding-left: ${indent + 15}px;">
                    <div class="d-flex justify-content-between align-items-center">
                        <span>${levelBadge}${folder.title}</span>
                        ${countBadge}
                    </div>
                </div>`;

        if (folderMap.has(folder.uid)) {
            folderMap.get(folder.uid).forEach(child => {
                renderFolder(child, level + 1);
            });
        }
    }

    rootFolders.forEach(folder => {
        renderFolder(folder, 1);
    });

    foldersList.innerHTML = html;

    document.querySelectorAll('#foldersList .folder-item').forEach(item => {
        item.addEventListener('click', (e) => {
            const now = Date.now();
            if (now - lastFolderClick < 300) {
                return;
            }
            lastFolderClick = now;

            const folderId = e.currentTarget.dataset.folderId;
            selectedFolder = folderId;

            document.querySelectorAll('#foldersList .folder-item').forEach(el => {
                el.classList.remove('selected-folder');
            });
            e.currentTarget.classList.add('selected-folder');

            showLoading('Filtering dashboards...', `Filtering by folder ID: ${folderId}`);

            setTimeout(() => {
                filterDashboards();
                hideLoading();
            }, 100);
        });
    });
}

function renderAlertFolders() {
    const alertFoldersList = document.getElementById('alertFoldersList');
    
    // Calculate alert counts per folder
    const folderAlertCounts = {};
    folderAlertCounts[0] = 0; // Initialize General folder count
    
    // Count alerts in each folder
    alerts.forEach(alert => {
        const folderId = alert.folderId || 0;
        folderAlertCounts[folderId] = (folderAlertCounts[folderId] || 0) + 1;
    });
    
    // Calculate total alerts
    const totalAlerts = alerts.length;
    
    let html = `<div class="list-group-item folder-item ${selectedAlertFolder === 'all' ? 'selected-folder' : ''}"
                    data-folder-id="all">All Folders <span class="badge bg-secondary ms-2">${totalAlerts}</span></div>`;

    html += `<div class="list-group-item folder-item ${selectedAlertFolder === '0' ? 'selected-folder' : ''}"
                data-folder-id="0">General <span class="badge bg-secondary ms-2">${folderAlertCounts[0] || 0}</span></div>`;

    const folderMap = new Map();
    const rootFolders = [];

    folders.forEach(folder => {
        if (folder.parentUid) {
            if (!folderMap.has(folder.parentUid)) {
                folderMap.set(folder.parentUid, []);
            }
            folderMap.get(folder.parentUid).push(folder);
        } else {
            rootFolders.push(folder);
        }
    });

    function renderFolder(folder, level) {
        const indent = level * 20;
        const levelBadge = `<span class="badge bg-primary me-2">L${level}</span>`;
        const alertCount = folderAlertCounts[folder.id] || 0;
        const countBadge = `<span class="badge bg-secondary ms-2">${alertCount}</span>`;

        html += `<div class="list-group-item folder-item ${selectedAlertFolder === folder.id.toString() ? 'selected-folder' : ''}"
                    data-folder-id="${folder.id}" style="padding-left: ${indent + 15}px;">
                    <div class="d-flex justify-content-between align-items-center">
                        <span>${levelBadge}${folder.title}</span>
                        ${countBadge}
                    </div>
                </div>`;

        if (folderMap.has(folder.uid)) {
            folderMap.get(folder.uid).forEach(child => {
                renderFolder(child, level + 1);
            });
        }
    }

    rootFolders.forEach(folder => {
        renderFolder(folder, 1);
    });

    alertFoldersList.innerHTML = html;

    document.querySelectorAll('#alertFoldersList .folder-item').forEach(item => {
        item.addEventListener('click', (e) => {
            const now = Date.now();
            if (now - lastFolderClick < 300) {
                return;
            }
            lastFolderClick = now;

            const folderId = e.currentTarget.dataset.folderId;
            selectedAlertFolder = folderId;

            document.querySelectorAll('#alertFoldersList .folder-item').forEach(el => {
                el.classList.remove('selected-folder');
            });
            e.currentTarget.classList.add('selected-folder');

            showLoading('Filtering alerts...', `Filtering by folder ID: ${folderId}`);

            setTimeout(() => {
                filterAlerts();
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
                        ${dashboard.tags && dashboard.tags.length > 0 ? 
                            `<div class="mt-1">
                                ${dashboard.tags.map(tag => `<span class="badge bg-warning me-1">${tag}</span>`).join('')}
                             </div>` : ''}
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

function renderAlerts() {
    if (filteredAlerts.length === 0) {
        alertsContainer.innerHTML = `
            <div class="text-center py-5 text-muted">
                <p>No alerts found matching your criteria</p>
            </div>
        `;
        return;
    }

    let html = '';
    filteredAlerts.forEach(alert => {
        const isSelected = selectedAlerts.has(alert.uid);
        const folderName = alert.folderTitle || 'General';

        html += `
            <div class="dashboard-item ${isSelected ? 'selected' : ''}" data-uid="${alert.uid}">
                <div class="d-flex justify-content-between align-items-center">
                    <div>
                        <div class="fw-medium">${alert.title}</div>
                        <div class="small text-muted">Folder: ${folderName}</div>
                    </div>
                    <div class="form-check">
                        <input class="form-check-input alert-checkbox" type="checkbox"
                            id="alert_check_${alert.uid}" data-uid="${alert.uid}" ${isSelected ? 'checked' : ''}>
                    </div>
                </div>
            </div>
        `;
    });

    alertsContainer.innerHTML = html;

    document.querySelectorAll('.alert-checkbox').forEach(checkbox => {
        checkbox.addEventListener('change', function() {
            const uid = this.dataset.uid;
            toggleAlertSelection(uid, this.checked);
        });
    });

    document.querySelectorAll('.dashboard-item[data-uid]').forEach(item => {
        if (item.querySelector('.alert-checkbox')) {
            item.addEventListener('click', function(e) {
                if (e.target.classList.contains('form-check-input')) return;

                const uid = this.dataset.uid;
                const checkbox = document.getElementById(`alert_check_${uid}`);
                if (checkbox) {
                    checkbox.checked = !checkbox.checked;
                    toggleAlertSelection(uid, checkbox.checked);
                }
            });
        }
    });
}

function filterDashboards() {
    console.log("Filtering dashboards...");
    console.log("Selected folder:", selectedFolder);
    console.log("Total dashboards:", dashboards.length);

    if (selectedFolder === 'all') {
        filteredDashboards = [...dashboards];
    } else {
        const folderId = parseInt(selectedFolder);
        filteredDashboards = dashboards.filter(dashboard => {
            return dashboard.folderId === folderId;
        });
    }

    if (searchQuery && searchQuery.length > 0) {
        const query = searchQuery.toLowerCase();
        filteredDashboards = filteredDashboards.filter(dashboard => {
            // Search in title
            if (dashboard.title.toLowerCase().includes(query)) {
                return true;
            }
            
            // Search in tags
            if (dashboard.tags && dashboard.tags.length > 0) {
                return dashboard.tags.some(tag => tag.toLowerCase().includes(query));
            }
            
            return false;
        });
    }

    console.log("Filtered dashboards:", filteredDashboards.length);
    renderDashboards();
}

function handleSearchInput() {
    searchQuery = searchDashboard.value.trim();
    filterDashboards();
}

function handleAlertSearchInput() {
    alertSearchQuery = searchAlerts.value.trim();
    filterAlerts();
}

function toggleDashboardSelection(uid, isSelected) {
    if (isSelected) {
        selectedDashboards.add(uid);
    } else {
        selectedDashboards.delete(uid);
    }

    updateSelectedCount();

    const dashboardItem = document.querySelector(`.dashboard-item[data-uid="${uid}"]`);
    if (dashboardItem) {
        if (isSelected) {
            dashboardItem.classList.add('selected');
        } else {
            dashboardItem.classList.remove('selected');
        }
    }
}

function toggleAlertSelection(uid, isSelected) {
    if (isSelected) {
        selectedAlerts.add(uid);
    } else {
        selectedAlerts.delete(uid);
    }

    const alertItem = document.querySelector(`.dashboard-item[data-uid="${uid}"]`);
    if (alertItem) {
        if (isSelected) {
            alertItem.classList.add('selected');
        } else {
            alertItem.classList.remove('selected');
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
    updateSelectedCount();
}

function clearDashboardSelection() {
    selectedDashboards.clear();
    document.querySelectorAll('.dashboard-checkbox').forEach(checkbox => {
        checkbox.checked = false;
    });

    renderDashboards();
    updateSelectedCount();
}

function selectAllAlerts() {
    filteredAlerts.forEach(alert => {
        selectedAlerts.add(alert.uid);
        const checkbox = document.getElementById(`alert_check_${alert.uid}`);
        if (checkbox) checkbox.checked = true;
    });

    renderAlerts();
    updateSelectedCount();
}

function clearAlertSelection() {
    selectedAlerts.clear();
    document.querySelectorAll('.alert-checkbox').forEach(checkbox => {
        checkbox.checked = false;
    });

    renderAlerts();
    updateSelectedCount();
}

function updateSelectedCount() {
    const dashboardCount = selectedDashboards.size;
    const alertCount = selectedAlerts.size;
    const totalCount = dashboardCount + alertCount;
    
    selectedCountElement.textContent = totalCount;
    
    // Force enable export button if configured, otherwise disable when no items selected
    exportBtn.disabled = appConfig.forceEnableExport ? false : (totalCount === 0);
    
    if (dashboardCount > 0 && alertCount > 0) {
        exportBtn.textContent = `Export ${dashboardCount} Dashboards & ${alertCount} Alerts`;
    } else if (dashboardCount > 0) {
        exportBtn.textContent = `Export ${dashboardCount} Dashboards`;
    } else if (alertCount > 0) {
        exportBtn.textContent = `Export ${alertCount} Alerts`;
    } else {
        exportBtn.textContent = 'Export Selected Items';
    }
}

function filterAlerts() {
    console.log("Filtering alerts...");
    console.log("Selected alert folder:", selectedAlertFolder);
    console.log("Total alerts:", alerts.length);

    // First filter by folder
    if (selectedAlertFolder === 'all') {
        filteredAlerts = [...alerts];
    } else {
        const folderId = parseInt(selectedAlertFolder);
        filteredAlerts = alerts.filter(alert => {
            return alert.folderId === folderId;
        });
    }

    // Then apply search filter if needed
    if (alertSearchQuery && alertSearchQuery.length > 0) {
        const query = alertSearchQuery.toLowerCase();
        filteredAlerts = filteredAlerts.filter(alert => {
            return alert.title.toLowerCase().includes(query) || 
                   (alert.folderTitle && alert.folderTitle.toLowerCase().includes(query));
        });
    }

    console.log("Filtered alerts:", filteredAlerts.length);
    renderAlerts();
}

function showExportResults(result) {
    // Create export results content
    let html = `
        <div>
            <p>Successfully exported <strong>${result.exportedDashboards}</strong> dashboards, 
               <strong>${result.exportedAlerts || 0}</strong> alerts, and 
               <strong>${result.exportedLibraries}</strong> linked library panels.</p>
            <p>Export path: <code>${result.exportPath}</code></p>
        </div>
    `;

    if (result.errors && result.errors.length > 0) {
        html += `
            <div class="alert alert-warning mt-3">
                <h5>Warnings/Errors</h5>
                <ul class="mb-0">
                    ${result.errors.map(error => `<li>${error}</li>`).join('')}
                </ul>
            </div>
        `;
    }

    exportResultContent.innerHTML = html;
    exportResultSection.style.display = 'block';
    
    // Add event listener to close button if not already added
    const closeBtn = document.getElementById('closeExportResults');
    if (closeBtn) {
        closeBtn.addEventListener('click', function() {
            exportResultSection.style.display = 'none';
        });
    }
}

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

    setTimeout(() => {
        const alertElement = document.getElementById(alertId);
        if (alertElement) {
            const bsAlert = new bootstrap.Alert(alertElement);
            bsAlert.close();
        }
    }, 5000);
}

async function loadConfig() {
    try {
        const response = await fetch('/api/config-status');
        if (!response.ok) throw new Error(`Failed to load config: ${response.statusText}`);

        const data = await response.json();
        appConfig.forceEnableZipExport = data.forceEnableZipExport || false;
        
        console.log("Loaded config:", appConfig);
        
        // Force enable ZIP export checkbox if configured
        if (appConfig.forceEnableZipExport) {
            if (exportAsZipCheck) {
                exportAsZipCheck.checked = true;
                exportAsZipCheck.disabled = true; // Disable so users can't uncheck it
            } else {
                console.error("exportAsZipCheck element not found!");
            }
        }
        
        // Update export button state after config is loaded
        updateSelectedCount();
    } catch (error) {
        console.warn('Failed to load config:', error.message);
        // Continue with default config if loading fails
    }
}

async function loadFolders() {
    try {
        const response = await fetch('/api/folders');
        if (!response.ok) throw new Error(`Failed to load folders: ${response.statusText}`);

        const data = await response.json();

        if (data.folders) {
            folders = data.folders;
            console.log("Loaded folders:", folders.length);
            console.log("Has nested structure:", data.hasNestedStructure);
            console.log("Debug info:", data.debug);

            showAlert(data.hasNestedStructure ? 'info' : 'warning',
                      `Folder info: ${data.debug}`);
        } else {
            folders = data;
            console.log("Loaded folders:", folders.length);
        }

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

async function exportSelectedDashboards() {
    if (selectedDashboards.size === 0 && selectedAlerts.size === 0) {
        showAlert('warning', 'Please select at least one dashboard or alert to export');
        return;
    }

    try {
        showLoading('Exporting dashboards, alerts, and linked libraries...');

        const dashboardUIDs = Array.from(selectedDashboards);
        const alertUIDs = Array.from(selectedAlerts);
        const includeAlerts = includeAlertsCheck.checked;
        const exportAsZip = exportAsZipCheck.checked;

        const response = await fetch('/api/export', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ 
                dashboardUIDs,
                alertUIDs,
                includeAlerts,
                exportAsZip
            })
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`Export failed: ${errorText}`);
        }

        // Check if response is a zip file
        const contentType = response.headers.get('content-type');
        if (exportAsZip && contentType && contentType.includes('application/zip')) {
            const blob = await response.blob();
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            // Try to get filename from Content-Disposition header
            const disposition = response.headers.get('content-disposition');
            let filename = 'grafana-export.zip';
            if (disposition && disposition.includes('filename=')) {
                filename = disposition.split('filename=')[1].replace(/"/g, '').trim();
            }
            a.download = filename;
            document.body.appendChild(a);
            a.click();
            setTimeout(() => {
                document.body.removeChild(a);
                window.URL.revokeObjectURL(url);
            }, 100);
            hideLoading();
            showAlert('success', 'ZIP export started. Check your downloads.');
            return;
        }

        const result = await response.json();
        hideLoading();
        showExportResults(result);
    } catch (error) {
        hideLoading();
        showAlert('error', `Export failed: ${error.message}`);
    }
}
