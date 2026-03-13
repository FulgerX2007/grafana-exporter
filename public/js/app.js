// ── DOM References ──
const foldersList = document.getElementById('foldersList');
const dashboardsContainer = document.getElementById('dashboardsContainer');
const searchDashboard = document.getElementById('searchDashboard');
const selectAllBtn = document.getElementById('selectAllBtn');
const clearSelectionBtn = document.getElementById('clearSelectionBtn');
const includeLibrariesCheck = document.getElementById('includeLibrariesCheck');
const includeAlertsCheck = document.getElementById('includeAlertsCheck');
const selectedDashCountEl = document.getElementById('selectedDashCount');
const selectedAlertCountEl = document.getElementById('selectedAlertCount');
const selectedFolderCountEl = document.getElementById('selectedFolderCount');
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
const sortOrder = document.getElementById('sortOrder');

// ── State ──
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
let currentSortOrder = 'alphabetical';
let expandedFolders = new Set();
let appConfig = { forceEnableZipExport: false };

// ── Init ──
document.addEventListener('DOMContentLoaded', initialize);

function initialize() {
    searchDashboard.addEventListener('input', handleSearchInput);
    selectAllBtn.addEventListener('click', selectAllDashboards);
    clearSelectionBtn.addEventListener('click', clearDashboardSelection);
    exportBtn.addEventListener('click', exportSelectedDashboards);
    sortOrder.addEventListener('change', changeSortOrder);

    searchAlerts.addEventListener('input', handleAlertSearchInput);
    selectAllAlertsBtn.addEventListener('click', selectAllAlerts);
    clearAlertsSelectionBtn.addEventListener('click', clearAlertSelection);

    document.getElementById('closeExportResults').addEventListener('click', () => {
        exportResultSection.style.display = 'none';
    });

    const savedSortOrder = localStorage.getItem('dashboardSortOrder') || 'alphabetical';
    currentSortOrder = savedSortOrder;
    sortOrder.value = savedSortOrder;

    loadConfig();
    loadFolders();
    loadDashboards();
    loadAlerts();
}

// ── Data Loading ──
async function loadDashboards() {
    try {
        showLoading('Loading dashboards...', 'Fetching from API...');
        const response = await fetch('/api/dashboards');
        if (!response.ok) throw new Error(`Failed to load dashboards: ${response.statusText}`);

        const data = await response.json();
        dashboards = data.dashboards || [];

        dashboards.forEach(d => {
            if (d.folderId === 0 && !d.folderTitle) d.folderTitle = 'General';
        });

        filteredDashboards = [...dashboards];

        setTimeout(() => {
            hideLoading();
            renderDashboards();
        }, 300);
    } catch (error) {
        hideLoading();
        showAlert('error', `Error loading dashboards: ${error.message}`);
    }
}

async function loadAlerts() {
    try {
        const response = await fetch('/api/alerts');
        if (!response.ok) throw new Error(`Failed to load alerts: ${response.statusText}`);

        const data = await response.json();
        alerts = data.alerts || [];
        filteredAlerts = [...alerts];

        setTimeout(() => {
            renderAlerts();
        }, 300);
    } catch (error) {
        showAlert('error', `Error loading alerts: ${error.message}`);
    }
}

async function loadFolders() {
    try {
        const response = await fetch('/api/folders');
        if (!response.ok) throw new Error(`Failed to load folders: ${response.statusText}`);

        const data = await response.json();

        if (data.folders) {
            folders = data.folders;
            showAlert(data.hasNestedStructure ? 'info' : 'warning', `Folder info: ${data.debug}`);
        } else {
            folders = data;
        }

        renderFolders();
    } catch (error) {
        showAlert('error', `Error loading folders: ${error.message}`);
    }
}

async function loadConfig() {
    try {
        const response = await fetch('/api/config-status');
        if (!response.ok) throw new Error(`Failed to load config: ${response.statusText}`);

        const data = await response.json();
        appConfig.forceEnableZipExport = data.forceEnableZipExport || false;

        if (appConfig.forceEnableZipExport && exportAsZipCheck) {
            exportAsZipCheck.checked = true;
            exportAsZipCheck.disabled = true;
        }

        updateSelectedCount();
    } catch (error) {
        console.warn('Failed to load config:', error.message);
    }
}

// ── Folder Rendering ──
function renderFolders() {
    renderDashboardFolders();
    renderAlertFolders();
}

function buildFolderTree(folderList) {
    const folderMap = new Map();
    const rootFolders = [];

    folderList.forEach(f => {
        if (f.parentUid) {
            if (!folderMap.has(f.parentUid)) folderMap.set(f.parentUid, []);
            folderMap.get(f.parentUid).push(f);
        } else {
            rootFolders.push(f);
        }
    });

    return { rootFolders, folderMap };
}

function renderDashboardFolders() {
    const { rootFolders, folderMap } = buildFolderTree(folders);

    let html = `
        <li class="folder-node">
            <div class="folder-row ${selectedFolder === 'all' ? 'active' : ''}" data-folder-id="all">
                <span class="folder-chevron empty"></span>
                <svg class="folder-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"></path></svg>
                <span class="folder-label">All Folders</span>
            </div>
        </li>
        <li class="folder-node">
            <div class="folder-row ${selectedFolder === '0' ? 'active' : ''}" data-folder-id="0">
                <span class="folder-chevron empty"></span>
                <svg class="folder-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"></path></svg>
                <span class="folder-label">General</span>
                ${getFolderCountBadge(0)}
            </div>
        </li>
    `;

    function renderFolder(folder, level) {
        const children = folderMap.get(folder.uid) || [];
        const hasChildren = children.length > 0;
        const isExpanded = expandedFolders.has(folder.id.toString());
        const count = folder.dashboardCount || 0;

        html += `<li class="folder-node">
            <div class="folder-row ${selectedFolder === folder.id.toString() ? 'active' : ''}" data-folder-id="${folder.id}" data-has-children="${hasChildren}">
                <span class="folder-chevron ${hasChildren ? (isExpanded ? 'open' : '') : 'empty'}" data-toggle="${folder.id}">
                    ${hasChildren ? '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><polyline points="9 18 15 12 9 6"></polyline></svg>' : ''}
                </span>
                <svg class="folder-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"></path></svg>
                <span class="folder-label">${folder.title}</span>
                ${count > 0 ? `<span class="folder-count">(${count})</span>` : ''}
            </div>`;

        if (hasChildren) {
            html += `<ul class="folder-children ${isExpanded ? '' : 'collapsed'}" data-parent="${folder.id}" style="max-height:${isExpanded ? '2000px' : '0'}">`;
            children.forEach(child => renderFolder(child, level + 1));
            html += '</ul>';
        }

        html += '</li>';
    }

    rootFolders.forEach(f => renderFolder(f, 1));
    foldersList.innerHTML = html;
    bindFolderEvents('#foldersList', 'dashboard');
}

function renderAlertFolders() {
    const alertFoldersList = document.getElementById('alertFoldersList');
    const { rootFolders, folderMap } = buildFolderTree(folders);

    const folderAlertCounts = {};
    alerts.forEach(a => {
        const fid = a.folderId || 0;
        folderAlertCounts[fid] = (folderAlertCounts[fid] || 0) + 1;
    });

    let html = `
        <li class="folder-node">
            <div class="folder-row ${selectedAlertFolder === 'all' ? 'active' : ''}" data-folder-id="all">
                <span class="folder-chevron empty"></span>
                <svg class="folder-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"></path></svg>
                <span class="folder-label">All Folders</span>
                <span class="folder-count">(${alerts.length})</span>
            </div>
        </li>
        <li class="folder-node">
            <div class="folder-row ${selectedAlertFolder === '0' ? 'active' : ''}" data-folder-id="0">
                <span class="folder-chevron empty"></span>
                <svg class="folder-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"></path></svg>
                <span class="folder-label">General</span>
                <span class="folder-count">(${folderAlertCounts[0] || 0})</span>
            </div>
        </li>
    `;

    function renderFolder(folder) {
        const children = folderMap.get(folder.uid) || [];
        const hasChildren = children.length > 0;
        const count = folderAlertCounts[folder.id] || 0;

        html += `<li class="folder-node">
            <div class="folder-row ${selectedAlertFolder === folder.id.toString() ? 'active' : ''}" data-folder-id="${folder.id}">
                <span class="folder-chevron ${hasChildren ? '' : 'empty'}">
                    ${hasChildren ? '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><polyline points="9 18 15 12 9 6"></polyline></svg>' : ''}
                </span>
                <svg class="folder-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"></path></svg>
                <span class="folder-label">${folder.title}</span>
                <span class="folder-count">(${count})</span>
            </div>`;

        if (hasChildren) {
            html += `<ul class="folder-children collapsed">`;
            children.forEach(child => renderFolder(child));
            html += '</ul>';
        }

        html += '</li>';
    }

    rootFolders.forEach(f => renderFolder(f));
    alertFoldersList.innerHTML = html;
    bindFolderEvents('#alertFoldersList', 'alert');
}

function getFolderCountBadge(folderId) {
    const count = dashboards.filter(d => d.folderId === folderId).length;
    return count > 0 ? `<span class="folder-count">(${count})</span>` : '';
}

function bindFolderEvents(selector, type) {
    const container = document.querySelector(selector);

    // Chevron toggle
    container.querySelectorAll('.folder-chevron:not(.empty)').forEach(chev => {
        chev.addEventListener('click', (e) => {
            e.stopPropagation();
            const li = chev.closest('.folder-node');
            const children = li.querySelector('.folder-children');
            if (!children) return;

            const folderId = chev.dataset.toggle || chev.closest('.folder-row').dataset.folderId;
            chev.classList.toggle('open');

            if (children.classList.contains('collapsed')) {
                children.classList.remove('collapsed');
                children.style.maxHeight = children.scrollHeight + 'px';
                expandedFolders.add(folderId);
            } else {
                children.classList.add('collapsed');
                children.style.maxHeight = '0';
                expandedFolders.delete(folderId);
            }
        });
    });

    // Folder selection
    container.querySelectorAll('.folder-row').forEach(row => {
        row.addEventListener('click', (e) => {
            if (e.target.closest('.folder-chevron:not(.empty)')) return;

            const now = Date.now();
            if (now - lastFolderClick < 300) return;
            lastFolderClick = now;

            const folderId = row.dataset.folderId;

            container.querySelectorAll('.folder-row').forEach(r => r.classList.remove('active'));
            row.classList.add('active');

            if (type === 'dashboard') {
                selectedFolder = folderId;
                filterDashboards();
            } else {
                selectedAlertFolder = folderId;
                filterAlerts();
            }
        });
    });
}

// ── Dashboard Rendering ──
function renderDashboards() {
    if (filteredDashboards.length === 0) {
        dashboardsContainer.innerHTML = `
            <div class="dashboards-empty">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="7" height="7"></rect><rect x="14" y="3" width="7" height="7"></rect><rect x="14" y="14" width="7" height="7"></rect><rect x="3" y="14" width="7" height="7"></rect></svg>
                <p>No dashboards found</p>
            </div>
        `;
        return;
    }

    let html = '';
    filteredDashboards.forEach(d => {
        const isSelected = selectedDashboards.has(d.uid);
        const folderName = d.folderTitle || 'General';
        const relTime = formatRelativeTime(d.updated);
        const panelCount = d.panels || '';

        html += `
            <div class="dashboard-card ${isSelected ? 'selected' : ''}" data-uid="${d.uid}">
                <span class="custom-check check-left">
                    <input type="checkbox" class="dashboard-checkbox" id="check_${d.uid}" data-uid="${d.uid}" ${isSelected ? 'checked' : ''}>
                    <span class="checkmark"></span>
                </span>
                <div class="dashboard-card-info">
                    <div class="dashboard-card-title">${d.title}</div>
                    <div class="dashboard-card-meta">${folderName}${panelCount ? ' (' + panelCount + ')' : ''}${relTime ? ' &middot; ' + relTime : ''}</div>
                    ${d.tags && d.tags.length > 0 ? `<div class="dashboard-card-tags">${d.tags.map(t => `<span class="tag-pill">${t}</span>`).join('')}</div>` : ''}
                </div>
                <span class="custom-check check-right">
                    <input type="checkbox" class="dashboard-checkbox-r" data-uid="${d.uid}" ${isSelected ? 'checked' : ''} tabindex="-1">
                    <span class="checkmark"></span>
                </span>
            </div>
        `;
    });

    dashboardsContainer.innerHTML = html;

    document.querySelectorAll('.dashboard-card').forEach(card => {
        card.addEventListener('click', function(e) {
            if (e.target.type === 'checkbox') return;
            const uid = this.dataset.uid;
            const cb = this.querySelector(`#check_${uid}`);
            cb.checked = !cb.checked;
            toggleDashboardSelection(uid, cb.checked);
        });
    });

    document.querySelectorAll('.dashboard-checkbox, .dashboard-checkbox-r').forEach(cb => {
        cb.addEventListener('change', function() {
            const uid = this.dataset.uid;
            toggleDashboardSelection(uid, this.checked);
        });
    });

    updateSelectedCount();
}

function renderAlerts() {
    if (filteredAlerts.length === 0) {
        alertsContainer.innerHTML = `
            <div class="dashboards-empty">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg>
                <p>No alerts found</p>
            </div>
        `;
        return;
    }

    let html = '';
    filteredAlerts.forEach(alert => {
        const isSelected = selectedAlerts.has(alert.uid);
        const folderName = alert.folderTitle || 'General';

        html += `
            <div class="dashboard-card ${isSelected ? 'selected' : ''}" data-uid="${alert.uid}">
                <span class="custom-check check-left">
                    <input type="checkbox" class="alert-checkbox" id="alert_check_${alert.uid}" data-uid="${alert.uid}" ${isSelected ? 'checked' : ''}>
                    <span class="checkmark"></span>
                </span>
                <div class="dashboard-card-info">
                    <div class="dashboard-card-title">${alert.title}</div>
                    <div class="dashboard-card-meta">${folderName}</div>
                </div>
                <span class="custom-check check-right">
                    <input type="checkbox" class="alert-checkbox-r" data-uid="${alert.uid}" ${isSelected ? 'checked' : ''} tabindex="-1">
                    <span class="checkmark"></span>
                </span>
            </div>
        `;
    });

    alertsContainer.innerHTML = html;

    alertsContainer.querySelectorAll('.dashboard-card').forEach(card => {
        card.addEventListener('click', function(e) {
            if (e.target.type === 'checkbox') return;
            const uid = this.dataset.uid;
            const cb = this.querySelector(`#alert_check_${uid}`);
            if (cb) {
                cb.checked = !cb.checked;
                toggleAlertSelection(uid, cb.checked);
            }
        });
    });

    alertsContainer.querySelectorAll('.alert-checkbox, .alert-checkbox-r').forEach(cb => {
        cb.addEventListener('change', function() {
            toggleAlertSelection(this.dataset.uid, this.checked);
        });
    });
}

// ── Filtering ──
function filterDashboards() {
    if (selectedFolder === 'all') {
        filteredDashboards = [...dashboards];
    } else {
        const folderId = parseInt(selectedFolder);
        filteredDashboards = dashboards.filter(d => d.folderId === folderId);
    }

    if (searchQuery) {
        const q = searchQuery.toLowerCase();
        filteredDashboards = filteredDashboards.filter(d =>
            d.title.toLowerCase().includes(q) ||
            (d.tags && d.tags.some(t => t.toLowerCase().includes(q)))
        );
    }

    applySorting();
    renderDashboards();
}

function filterAlerts() {
    if (selectedAlertFolder === 'all') {
        filteredAlerts = [...alerts];
    } else {
        const folderId = parseInt(selectedAlertFolder);
        filteredAlerts = alerts.filter(a => a.folderId === folderId);
    }

    if (alertSearchQuery) {
        const q = alertSearchQuery.toLowerCase();
        filteredAlerts = filteredAlerts.filter(a =>
            a.title.toLowerCase().includes(q) ||
            (a.folderTitle && a.folderTitle.toLowerCase().includes(q))
        );
    }

    renderAlerts();
}

function handleSearchInput() {
    searchQuery = searchDashboard.value.trim();
    filterDashboards();
}

function handleAlertSearchInput() {
    alertSearchQuery = searchAlerts.value.trim();
    filterAlerts();
}

// ── Selection ──
function toggleDashboardSelection(uid, isSelected) {
    if (isSelected) {
        selectedDashboards.add(uid);
    } else {
        selectedDashboards.delete(uid);
    }

    // Sync both checkboxes
    const card = dashboardsContainer.querySelector(`.dashboard-card[data-uid="${uid}"]`);
    if (card) {
        card.classList.toggle('selected', isSelected);
        card.querySelectorAll('input[type="checkbox"]').forEach(cb => cb.checked = isSelected);
    }

    updateSelectedCount();
}

function toggleAlertSelection(uid, isSelected) {
    if (isSelected) {
        selectedAlerts.add(uid);
    } else {
        selectedAlerts.delete(uid);
    }

    const card = alertsContainer.querySelector(`.dashboard-card[data-uid="${uid}"]`);
    if (card) {
        card.classList.toggle('selected', isSelected);
        card.querySelectorAll('input[type="checkbox"]').forEach(cb => cb.checked = isSelected);
    }

    updateSelectedCount();
}

function selectAllDashboards() {
    filteredDashboards.forEach(d => selectedDashboards.add(d.uid));
    renderDashboards();
    updateSelectedCount();
}

function clearDashboardSelection() {
    selectedDashboards.clear();
    renderDashboards();
    updateSelectedCount();
}

function selectAllAlerts() {
    filteredAlerts.forEach(a => selectedAlerts.add(a.uid));
    renderAlerts();
    updateSelectedCount();
}

function clearAlertSelection() {
    selectedAlerts.clear();
    renderAlerts();
    updateSelectedCount();
}

function updateSelectedCount() {
    const dashCount = selectedDashboards.size;
    const alertCount = selectedAlerts.size;
    const totalCount = dashCount + alertCount;

    selectedDashCountEl.textContent = dashCount;
    selectedAlertCountEl.textContent = alertCount;

    // Count unique folders
    const folderIds = new Set();
    dashboards.filter(d => selectedDashboards.has(d.uid)).forEach(d => folderIds.add(d.folderId));
    selectedFolderCountEl.textContent = folderIds.size;

    exportBtn.disabled = appConfig.forceEnableExport ? false : (totalCount === 0);
    exportBtn.textContent = `Export (${totalCount})`;

}

// ── Sorting ──
function applySorting() {
    switch (currentSortOrder) {
        case 'recently-updated':
            filteredDashboards.sort((a, b) => new Date(b.updated || 0) - new Date(a.updated || 0));
            break;
        case 'oldest-updated':
            filteredDashboards.sort((a, b) => new Date(a.updated || 0) - new Date(b.updated || 0));
            break;
        default:
            filteredDashboards.sort((a, b) => a.title.localeCompare(b.title));
    }
}

function changeSortOrder() {
    currentSortOrder = sortOrder.value;
    localStorage.setItem('dashboardSortOrder', currentSortOrder);
    filterDashboards();
}

// ── Export ──
async function exportSelectedDashboards() {
    if (selectedDashboards.size === 0 && selectedAlerts.size === 0) {
        showAlert('warning', 'Please select at least one dashboard or alert to export');
        return;
    }

    try {
        showLoading('Exporting dashboards, alerts, and linked libraries...');

        const response = await fetch('/api/export', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                dashboardUIDs: Array.from(selectedDashboards),
                alertUIDs: Array.from(selectedAlerts),
                includeAlerts: includeAlertsCheck.checked,
                exportAsZip: exportAsZipCheck.checked
            })
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`Export failed: ${errorText}`);
        }

        const contentType = response.headers.get('content-type');
        if (exportAsZipCheck.checked && contentType && contentType.includes('application/zip')) {
            const blob = await response.blob();
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
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

function showExportResults(result) {
    let html = `
        <p>Successfully exported <strong>${result.exportedDashboards}</strong> dashboards,
           <strong>${result.exportedAlerts || 0}</strong> alerts, and
           <strong>${result.exportedLibraries}</strong> linked library panels.</p>
        <p>Export path: <code>${result.exportPath}</code></p>
    `;

    if (result.errors && result.errors.length > 0) {
        html += `
            <div class="export-result-warnings">
                <h4>Warnings/Errors</h4>
                <ul>${result.errors.map(e => `<li>${e}</li>`).join('')}</ul>
            </div>
        `;
    }

    exportResultContent.innerHTML = html;
    exportResultSection.style.display = 'block';
}

// ── Utilities ──
function formatRelativeTime(dateString) {
    if (!dateString) return '';
    try {
        const diff = Math.floor((Date.now() - new Date(dateString)) / 1000);
        if (diff < 60) return 'just now';
        if (diff < 3600) { const m = Math.floor(diff / 60); return `${m}m ago`; }
        if (diff < 86400) { const h = Math.floor(diff / 3600); return `${h}h ago`; }
        if (diff < 2592000) { const d = Math.floor(diff / 86400); return `${d}d ago`; }
        if (diff < 31536000) { const mo = Math.floor(diff / 2592000); return `${mo}mo ago`; }
        const y = Math.floor(diff / 31536000); return `${y}y ago`;
    } catch {
        return '';
    }
}

function showLoading(message, debug = '') {
    loadingText.textContent = message || 'Processing...';
    debugInfo.textContent = debug;
    loadingOverlay.classList.add('visible');
}

function hideLoading() {
    loadingOverlay.classList.remove('visible');
}

function showAlert(type, message) {
    const id = 'toast_' + Date.now();
    const icons = {
        info: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>',
        warning: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg>',
        error: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg>',
        success: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 11-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline></svg>'
    };

    const html = `
        <div id="${id}" class="alert-toast ${type}">
            ${icons[type] || ''}
            <span>${message}</span>
            <button class="close-btn" onclick="this.parentElement.remove()">&times;</button>
        </div>
    `;

    alertContainer.insertAdjacentHTML('beforeend', html);

    setTimeout(() => {
        const el = document.getElementById(id);
        if (el) el.remove();
    }, 6000);
}
