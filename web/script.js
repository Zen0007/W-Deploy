'use strict';

// Helpers
const $ = (s, r) => (r || document).querySelector(s);
const $$ = (s, r) => Array.from((r || document).querySelectorAll(s));
const cap = s => s[0].toUpperCase() + s.slice(1);
const esc = s => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const pad2 = n => String(n).padStart(2, '0');
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
function fmtDateTime(ts) {
    const d = new Date(ts);
    return d.getUTCDate() + ' ' + MONTHS[d.getUTCMonth()] + ' ' + d.getUTCFullYear() + ', ' + pad2(d.getUTCHours()) + ':' + pad2(d.getUTCMinutes());
}
function dayKey(ts) { return new Date(ts).toISOString().slice(0, 10); }
function rel(ts) {
    const diff = Date.now() - ts;
    if (diff < 60e3) return 'just now';
    const m = Math.floor(diff / 60e3);
    if (m < 60) return m + 'm ago';
    const h = Math.floor(m / 60);
    if (h < 24) return h + 'h ' + (m % 60) + 'm ago';
    return Math.floor(h / 24) + 'd ago';
}
function relHours(h) {
    if (h < 1) return 'just now';
    if (h < 24) return h + ' hour' + (h > 1 ? 's' : '') + ' ago';
    const d = Math.round(h / 24);
    return d + ' day' + (d > 1 ? 's' : '') + ' ago';
}
function copyText(v) { try { if (navigator.clipboard) navigator.clipboard.writeText(v).catch(function () { }); } catch (_) { } }

// Inline SVG icons (stroke 1.5, currentColor, Lucide-like) + provider logos (fill, currentColor)
const PATHS = {
    x: '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
    check: '<path d="M20 6 9 17l-5-5"/>',
    info: '<circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/>',
    'circle-check': '<path d="M21.801 10A10 10 0 1 1 17 3.335"/><path d="m9 11 3 3L22 4"/>',
    'alert-triangle': '<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/>',
    'git-branch': '<line x1="6" x2="6" y1="3" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/>'
};
function svgIcon(name, cls) {
    return '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="' + cls + '" aria-hidden="true">' + PATHS[name] + '</svg>';
}
const GH_PATH = 'M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12';
const GL_PATH = 'm23.6004 9.5927-.0337-.0862L20.3.9814a.851.851 0 0 0-.3362-.405.8748.8748 0 0 0-.9997.0539.8748.8748 0 0 0-.29.4399l-2.2055 6.748H7.5375l-2.2057-6.748a.8573.8573 0 0 0-.29-.4412.8748.8748 0 0 0-.9997-.0537.8585.8585 0 0 0-.3362.4049L.4332 9.5015l-.0325.0862a6.0657 6.0657 0 0 0 2.0119 7.0105l.0113.0087.03.0213 4.976 3.7264 2.462 1.8633 1.4995 1.1321a1.0085 1.0085 0 0 0 1.2197 0l1.4995-1.1321 2.4619-1.8633 5.006-3.7489.0125-.01a6.0682 6.0682 0 0 0 2.0094-7.003z';
function ghLogo(cls) { return '<svg viewBox="0 0 24 24" fill="currentColor" class="' + cls + '" aria-hidden="true"><path d="' + GH_PATH + '"/></svg>'; }
function glLogo(cls) { return '<svg viewBox="0 0 24 24" fill="currentColor" class="' + cls + '" aria-hidden="true"><path d="' + GL_PATH + '"/></svg>'; }
function providerLogo(p, cls) { return p === 'github' ? ghLogo(cls) : glLogo(cls); }
function providerName(p) { return p === 'github' ? 'GitHub' : 'GitLab'; }

// ─── State & API layer ───────────────────────────────────────────────────────

// Live data: replaced on every API call
let REPOS = [];
let CONTAINERS = [];   // maps to /api/apps
let DEPLOYS = [];   // maps to /api/apps/{id}/deployments (aggregated)

// Local UI state
let connections = {
    github: { connected: false, account: '', scopes: '', date: '', needsReconnect: false },
    gitlab: { connected: false, account: '', scopes: '', date: '', needsReconnect: false }
};
let env = 'Production';
let ui = { confirm: null };
let envVars = [];          // per-app env vars, loaded on demand
let dirty = false;
let current = 'overview';
let currentUser = null;     // set by checkAuth()
let currentAppId = null;    // used for env-var panel context
const SESSION_TOKEN_KEY = 'paas_session';
let sessionToken = '';
try {
    sessionToken = sessionStorage.getItem(SESSION_TOKEN_KEY) || '';
} catch (err) {
    console.warn('Unable to restore the session token:', err);
}

function closeAllResourceWs() {
    if (_resWsCpu) { _resWsCpu.close(); _resWsCpu = null; }
    if (_resWsRam) { _resWsRam.close(); _resWsRam = null; }
    if (_resWsDisk) { _resWsDisk.close(); _resWsDisk = null; }
    stopContainerStatsMonitoring();
}


// ─── Fetch helpers ───────────────────────────────────────────────────────────
let isRefreshing = false;
let refreshSubscribers = [];

function onRefreshed(token) {
    refreshSubscribers.forEach(cb => cb(token));
    refreshSubscribers = [];
}

const API_BASE = 'https://api.wdeploy.my.id';
/** Centralised fetch with session authentication and JSON parsing. */
async function apiFetch(url, opts) {
    const fullURL = url.startsWith('http') ? url : API_BASE + url;
    const options = Object.assign({ credentials: 'include', headers: {} }, opts || {});
    options.headers = new Headers(options.headers);

    if (sessionToken && !options.headers.has('Authorization')) {
        options.headers.set('Authorization', 'Bearer ' + sessionToken);
    }

    if (options.body && !(options.body instanceof FormData) && typeof options.body === 'object') {
        options.body = JSON.stringify(options.body);
        options.headers.set('Content-Type', 'application/json');
    }

    let res = await fetch(fullURL, options);

    // === 401 handling ===
    if (res.status === 401) {
        // 1) Prioritas: error khusus GitHub perlu reconnect
        let errBody = null;
        try { errBody = await res.clone().json(); } catch (_) { }
        if (errBody && errBody.error === 'GITHUB_REAUTH_REQUIRED') {
            connections.github.connected = false;
            connections.github.needsReconnect = true;
            const err = new Error(errBody.message || 'GitHub reconnect required');
            err.code = 'GITHUB_REAUTH_REQUIRED';
            err.status = 401;
            throw err;
        }

        // 2) 401 lainnya = session W-deploy → coba refresh session
        if (!url.includes('/api/auth/refresh') && !url.includes('/api/auth/github')) {
            if (!isRefreshing) {
                isRefreshing = true;
                try {
                    const refreshRes = await fetch(API_BASE + '/api/auth/refresh', {
                        method: 'POST',
                        credentials: 'include'
                    });
                    if (refreshRes.ok) {
                        const data = await refreshRes.json();
                        if (data.token) {
                            sessionToken = data.token;
                            try { sessionStorage.setItem(SESSION_TOKEN_KEY, sessionToken); } catch (_) { }
                        }
                        isRefreshing = false;
                        onRefreshed(sessionToken);
                        closeAllResourceWs();
                        startResourceMonitoring();
                        startContainerStatsMonitoring();
                        return await apiFetch(url, opts);
                    } else {
                        throw new Error('Refresh token expired');
                    }
                } catch (e) {
                    isRefreshing = false;
                    sessionToken = '';
                    try { sessionStorage.removeItem(SESSION_TOKEN_KEY); } catch (_) { }
                    closeAllResourceWs();
                    toast('danger', 'Session expired', 'Sesi Anda telah berakhir. Silakan login kembali.');
                    showLoginPrompt();
                    const err = new Error('Unauthorized');
                    err.status = 401;
                    throw err;
                }
            } else {
                // Tunggu refresh yang sedang berjalan, lalu retry; tetap deteksi GITHUB_REAUTH_REQUIRED
                return new Promise(resolve => {
                    refreshSubscribers.push((newToken) => {
                        options.headers.set('Authorization', 'Bearer ' + newToken);
                        resolve((async () => {
                            const r = await fetch(fullURL, options);
                            if (r.status === 401) {
                                let body = null;
                                try { body = await r.clone().json(); } catch (_) { }
                                if (body && body.error === 'GITHUB_REAUTH_REQUIRED') {
                                    connections.github.connected = false;
                                    connections.github.needsReconnect = true;
                                    const err = new Error(body.message || 'GitHub reconnect required');
                                    err.code = 'GITHUB_REAUTH_REQUIRED';
                                    err.status = 401;
                                    throw err;
                                }
                            }
                            if (!r.ok) {
                                const err = new Error(r.statusText || ('HTTP ' + r.status));
                                err.status = r.status;
                                throw err;
                            }
                            const ct = r.headers.get('Content-Type') || '';
                            return ct.includes('application/json') ? r.json() : r;
                        })());
                    });
                });
            }
        }
    }

    if (!res.ok) {
        let msg = res.statusText || ('HTTP ' + res.status);
        try { const j = await res.clone().json(); msg = j.error || j.message || msg; } catch (_) { }
        console.error('[api] error response:', res.status, msg, url);
        const err = new Error(msg);
        err.status = res.status;
        throw err;
    }

    const ct = res.headers.get('Content-Type') || '';
    return ct.includes('application/json') ? res.json() : res;
}

function captureOAuthSession() {
    const rawHash = window.location.hash;
    const rawSearch = window.location.search;

    let token = '';

    // 1. Query string biasa: /?token=abc
    if (rawSearch) {
        const sp = new URLSearchParams(rawSearch.slice(1));
        token = sp.get('token') || sp.get('access_token') || '';
    }

    // 2. Hash: #/overview?token=abc  |  #token=abc  |  #?token=abc
    if (!token && rawHash) {
        const hash = rawHash.slice(1);
        const qIdx = hash.indexOf('?');
        const hQuery = qIdx >= 0 ? hash.slice(qIdx + 1)
            : hash.indexOf('=') >= 0 ? hash : '';
        if (hQuery) {
            const hp = new URLSearchParams(hQuery);
            token = hp.get('token') || hp.get('access_token') || '';
        }
    }

    if (!token) return;

    sessionToken = token;
    try { sessionStorage.setItem(SESSION_TOKEN_KEY, token); } catch (_) { }

    // Bersihkan token dari URL bar
    const cleanHash = rawHash.replace(/[?&](token|access_token)=[^&]+/g, '').replace(/\?$/, '');
    history.replaceState(null, '', location.pathname + rawSearch + (cleanHash || '#/overview'));
}
// ─── Auth ────────────────────────────────────────────────────────────────────

/**
 * return:
 *   'authenticated'  → user login
 *   'unauthorized'   → 401 (belum login / token expired)
 *   'error'          → backend down / network error
 */
let sessionTimeoutTimer = null;
let refreshTimeoutTimer = null;    // ← BARU: timer untuk refresh token expiry
let refreshCheckInterval = null;   // ← BARU: interval backup untuk deteksi sleep/wake


async function checkAuth() {
    try {
        const user = await apiFetch('/api/auth/me');
        currentUser = user;
        updateUserProfileUI(user);

        const ghLogin = user.github_login || user.login || '';
        if (ghLogin) {
            connections.github = {
                connected: true,
                account: '@' + ghLogin,
                scopes: user.github_scopes || 'repo, admin:repo_hook',
                date: user.github_connected_at ? fmtDateTime(new Date(user.github_connected_at).getTime()) : fmtDateTime(Date.now())
            };
        }

        // ─── Setup Access Token Proactive Refresh (sudah ada) ───
        if (user.expires_at) {
            const expireTime = new Date(user.expires_at).getTime();
            const timeLeft = expireTime - Date.now();

            if (sessionTimeoutTimer) clearTimeout(sessionTimeoutTimer);

            if (timeLeft > 0) {
                const refreshIn = Math.max(5000, timeLeft - 60000);
                sessionTimeoutTimer = setTimeout(async () => {
                    console.log("[auth] Proactively refreshing access token...");
                    try {
                        const res = await fetch(API_BASE + '/api/auth/refresh', { method: 'POST', credentials: 'include' });
                        if (res.ok) {
                            const data = await res.json();
                            if (data.token) {
                                sessionToken = data.token;
                                sessionStorage.setItem(SESSION_TOKEN_KEY, sessionToken);
                            }
                            closeAllResourceWs();
                            startResourceMonitoring();
                            startContainerStatsMonitoring();
                            checkAuth();
                        } else {
                            closeAllResourceWs();
                            toast('danger', 'Session expired', 'Silakan login kembali.');
                            showLoginPrompt();
                        }
                    } catch (e) {
                        console.warn('Proactive refresh failed', e);
                    }
                }, refreshIn);
            }
        }

        // ─── BARU: Setup Refresh Token Auto-Logout ───
        setupRefreshTokenAutoLogout(user.refresh_expires_at);

        return 'authenticated';
    } catch (err) {
        currentUser = null;
        return err.status === 401 ? 'unauthorized' : 'error';
    }
}

function setupRefreshTokenAutoLogout(refreshExpiresAtISO) {
    // Bersihkan timer lama
    if (refreshTimeoutTimer) { clearTimeout(refreshTimeoutTimer); refreshTimeoutTimer = null; }
    if (refreshCheckInterval) { clearInterval(refreshCheckInterval); refreshCheckInterval = null; }

    if (!refreshExpiresAtISO) {
        console.log('[auth] No refresh_expires_at provided — skipping auto logout timer');
        return;
    }

    const expireTime = new Date(refreshExpiresAtISO).getTime();
    if (isNaN(expireTime)) {
        console.warn('[auth] Invalid refresh_expires_at:', refreshExpiresAtISO);
        return;
    }

    // Fungsi yang dijalankan saat refresh token benar-benar expired
    const doAutoLogout = async () => {
        console.log('[auth] Refresh token expired — auto logout');
        if (sessionTimeoutTimer) clearTimeout(sessionTimeoutTimer);
        if (refreshTimeoutTimer) clearTimeout(refreshTimeoutTimer);
        if (refreshCheckInterval) clearInterval(refreshCheckInterval);
        sessionTimeoutTimer = refreshTimeoutTimer = refreshCheckInterval = null;

        // Hentikan WebSocket
        closeAllResourceWs();

        // Panggil endpoint logout untuk hapus cookie di backend (best-effort)
        try {
            await fetch(API_BASE + '/api/auth/logout', {
                method: 'POST',
                credentials: 'include',
            });
        } catch (_) { /* ignore */ }

        // Bersihkan state frontend
        sessionToken = '';
        try { sessionStorage.removeItem(SESSION_TOKEN_KEY); } catch (_) { }

        toast('danger', 'Session expired', 'Sesi Anda telah berakhir. Silakan login kembali.');
        showLoginPrompt();
    };

    const timeLeft = expireTime - Date.now();

    if (timeLeft <= 0) {
        // Sudah expired — langsung logout tanpa menunggu
        console.log('[auth] Refresh token already expired — logging out immediately');
        doAutoLogout();
        return;
    }

    // Set timer untuk auto-logout tepat waktu.
    // setTimeout max = 2^31-1 ms (~24.8 hari). Jika lebih, pecah jadi chunk.
    const MAX_TIMEOUT = 2147483647;
    if (timeLeft <= MAX_TIMEOUT) {
        refreshTimeoutTimer = setTimeout(doAutoLogout, timeLeft);
        console.log('[auth] Auto-logout scheduled in', Math.round(timeLeft / 1000), 'seconds');
    } else {
        // Untuk kasus ekstrem (>24 hari) — cek secara bertahap
        console.log('[auth] Refresh token lifetime >24 days — using staged timer');
        const stage = () => {
            const remaining = expireTime - Date.now();
            if (remaining <= 0) { doAutoLogout(); return; }
            if (remaining <= MAX_TIMEOUT) {
                refreshTimeoutTimer = setTimeout(doAutoLogout, remaining);
            } else {
                refreshTimeoutTimer = setTimeout(stage, MAX_TIMEOUT);
            }
        };
        stage();
    }

    // Backup check setiap 60 detik — menangani kasus laptop sleep/wake
    // di mana setTimeout bisa saja "terlewat" tanpa fire.
    refreshCheckInterval = setInterval(() => {
        if (Date.now() >= expireTime) {
            doAutoLogout();
        }
    }, 60000);
}

function updateUserProfileUI(user) {
    const initials = (user.name || user.login || 'U').slice(0, 2).toUpperCase();
    const avatarEl = $('#sidebarAvatar');
    const nameEl = $('#sidebarName');
    const roleEl = $('#sidebarRole');
    if (avatarEl) avatarEl.textContent = initials;
    if (nameEl) nameEl.textContent = user.name || user.login || 'Unknown';
    if (roleEl) roleEl.textContent = user.role || 'Owner';
}

async function logoutUser() {
    // Bersihkan semua timer dulu
    if (sessionTimeoutTimer) { clearTimeout(sessionTimeoutTimer); sessionTimeoutTimer = null; }
    if (refreshTimeoutTimer) { clearTimeout(refreshTimeoutTimer); refreshTimeoutTimer = null; }
    if (refreshCheckInterval) { clearInterval(refreshCheckInterval); refreshCheckInterval = null; }
    closeAllResourceWs();

    // Pakai fetch mentah — bypass apiFetch 401 retry loop.
    // Timeout 5 detik supaya tidak hang.
    try {
        const controller = new AbortController();
        const t = setTimeout(() => controller.abort(), 5000);
        await fetch(API_BASE + '/api/auth/logout', {
            method: 'POST',
            credentials: 'include',
            signal: controller.signal,
        });
        clearTimeout(t);
    } catch (_) { /* ignore — tetap bersihkan lokal */ }

    // Selalu bersihkan state lokal, apapun hasil API
    sessionToken = '';
    try { sessionStorage.removeItem(SESSION_TOKEN_KEY); } catch (_) { }
    currentUser = null;

    // Hapus overlay login kalau ada supaya tidak bentrok dengan reload
    const ov = document.getElementById('loginOverlay');
    if (ov) ov.remove();

    window.location.reload();
}

// ─── GitHub Repos ─────────────────────────────────────────────────────────────

async function fetchRepos() {
    try {
        const data = await apiFetch('/api/github/repos');

        // Sukses → bersihkan state reconnect (mungkin dipulihkan backend via refresh token)
        if (connections.github.needsReconnect) {
            connections.github.needsReconnect = false;
            if (!connections.github.connected) connections.github.connected = true;
            if (current === 'integrations') renderProviderCards();
        }

        REPOS = (Array.isArray(data) ? data : []).map(function (r) {
            return {
                name: r.full_name || r.name,
                provider: 'github',
                lang: r.language || 'Unknown',
                color: langColor(r.language),
                updatedH: r.updated_at ? Math.max(0, Math.round((Date.now() - new Date(r.updated_at).getTime()) / 36e5)) : 0,
                defaultBranch: r.default_branch || 'main'
            };
        });
        renderRepoList();
        console.log('[repos] loaded', REPOS.length);
    } catch (err) {
        if (err.code === 'GITHUB_REAUTH_REQUIRED') {
            toast('danger', 'GitHub reconnect required', err.message);
            if (current === 'integrations') renderProviderCards();
            return;
        }
        console.error('[repos] fetch error', err.status, err.message);
        toast('danger', 'Repos error', err.message);
    }
}

function langColor(lang) {
    const MAP = {
        TypeScript: '#3178C6', JavaScript: '#F7DF1E', Go: '#00ADD8', Python: '#3572A5',
        Rust: '#DEA584', Ruby: '#701516', Java: '#B07219', Kotlin: '#A97BFF', HCL: '#844FBA',
        MDX: '#FCB32C', HTML: '#E34C26', CSS: '#563D7C', Shell: '#89E051'
    };
    return MAP[lang] || '#71717A';
}

// ─── Apps / Containers ───────────────────────────────────────────────────────

/** Map a backend App object to the CONTAINERS row shape. */
function normaliseApp(a) {
    const slug = a.slug || a.name || '';
    const fallbackUrl = slug ? 'https://' + slug + '.wdeploy.my.id' : '';
    return {
        id: a.id,
        slug: slug,
        sourceType: a.source_type || 'github',
        service: a.name || a.slug || 'app',
        url: a.url || fallbackUrl,
        image: a.image_tag || a.image || a.docker_image || '—',
        status: normaliseStatus(a.status),
        cpu: a.cpu ? a.cpu + '%' : '—',
        mem: a.memory ? a.memory : '—',
        uptime: a.uptime || '—',
        region: a.region || 'local'
    };
}

function normaliseStatus(s) {
    const MAP = {
        running: 'Running', stopped: 'Stopped', failed: 'Failed',
        restarting: 'Restarting', building: 'Building', pending: 'Building',
        deploying: 'Building'
    };
    return MAP[(s || '').toLowerCase()] || 'Stopped';
}

async function fetchApps() {
    try {
        const data = await apiFetch('/api/apps');
        const list = Array.isArray(data) ? data : (data && Array.isArray(data.apps) ? data.apps : []);
        CONTAINERS = list.map(normaliseApp);
        renderRoute();
    } catch (err) {
        console.error('[apps] fetch error:', err);
        toast('danger', 'Apps error', err.message);
    }
}

async function deleteApp(id) {
    try {
        await apiFetch('/api/apps/' + id, { method: 'DELETE' });
        toast('success', 'App deleted', id + ' has been removed.');
        await fetchApps();
    } catch (err) {
        toast('danger', 'Delete failed', err.message);
    }
}

async function redeployApp(id) {
    try {
        // Optimistic: tandai building dulu
        const i = CONTAINERS.findIndex(function (c) { return c.id === id; });
        if (i > -1) { CONTAINERS[i].status = 'Building'; renderRoute(); }

        const res = await apiFetch('/api/apps/' + id + '/redeploy', { method: 'POST' });
        toast('success', 'Redeploy triggered', 'New build queued for ' + id + '.');

        // Tampilkan progress panel — status build visible
        const c = CONTAINERS.find(function (x) { return String(x.id) === String(id); });
        const slug = c ? c.slug : 'app';
        const depId = res && res.deployment_id ? res.deployment_id : null;
        dpStart(id, slug, depId, c ? c.sourceType : 'upload');

    } catch (err) {
        toast('danger', 'Redeploy failed', err.message);
        const i = CONTAINERS.findIndex(function (c) { return c.id === id; });
        if (i > -1) { CONTAINERS[i].status = 'Running'; renderRoute(); }
    }
}

// ─── Deployments ─────────────────────────────────────────────────────────────
function fmtDuration(sec) {
    if (sec == null || isNaN(sec)) return '—';
    sec = Math.floor(sec);
    if (sec < 60) return sec + 's';
    const m = Math.floor(sec / 60);
    const s = sec % 60;
    if (m < 60) return m + 'm ' + s + 's';
    const h = Math.floor(m / 60);
    const rm = m % 60;
    return h + 'h ' + rm + 'm';
}

function normaliseDeploy(d) {
    let rawStatus = cap(d.status || 'unknown');
    if (rawStatus === 'Running') rawStatus = 'Success';

    // ── Hitung durasi ──────────────────────────────────────────
    let duration = '—';
    if (typeof d.duration_sec === 'number' && d.duration_sec > 0) {
        duration = fmtDuration(d.duration_sec);
    } else if (d.started_at && d.finished_at) {
        const diffMs = new Date(d.finished_at).getTime() - new Date(d.started_at).getTime();
        if (diffMs >= 0) duration = fmtDuration(Math.floor(diffMs / 1000));
    }
    // ───────────────────────────────────────────────────────────

    return {
        id: d.id,
        sha: (d.commit_sha || d.sha || '').slice(0, 7),
        message: d.commit_message || d.message || '—',
        repo: d.repo_url || d.repo || '—',
        branch: d.branch || 'main',
        provider: 'github',
        status: rawStatus,
        duration: duration,
        ts: d.created_at ? new Date(d.created_at).getTime() : Date.now()
    };
}

/** Aggregate deployments from all apps (for Deployments page). */
async function fetchAllDeploys() {
    if (!CONTAINERS.length) return;
    try {
        const results = await Promise.all(
            CONTAINERS.map(async function (c) {
                try {
                    const res = await apiFetch('/api/apps/' + c.id + '/deployments');
                    return Array.isArray(res) ? res : (res && Array.isArray(res.deployments) ? res.deployments : []);
                } catch {
                    return [];
                }
            })
        );
        DEPLOYS = results
            .flat()
            .map(normaliseDeploy)
            .sort(function (a, b) { return b.ts - a.ts; });
        renderDeploymentsPage();
        renderOverview();
    } catch (err) {
        console.error('[deploys] fetch error:', err);
        toast('danger', 'Deploys error', err.message);
    }
}
// ─── Deploy-modal env state ─────────────────────────────────────────
let deployEnvVars = [];  // {k, v}

// ─── Env draft (mirror ke storage) ──────────────────────────────────
// Setiap perubahan baris env disalin ke storage. Kalau state di memori
// ternyata hilang/kosong saat user klik Add, baris dipulihkan dari sini.
// Ganti ke window.sessionStorage kalau tidak mau draft bertahan setelah tab ditutup.
const ENV_DRAFT_STORE = (function () { try { return window.localStorage; } catch (e) { return null; } })();
const ENV_DRAFT_PREFIX = 'wdeploy:env-draft:';

function envDraftRead(ctx) {
    if (!ENV_DRAFT_STORE) return null;
    try {
        const raw = ENV_DRAFT_STORE.getItem(ENV_DRAFT_PREFIX + ctx);
        const arr = raw ? JSON.parse(raw) : null;
        if (!Array.isArray(arr)) return null;
        return arr.map(function (r) { return { k: String((r && r.k) || ''), v: String((r && r.v) || '') }; });
    } catch (e) { return null; }
}
function envDraftWrite(ctx, rows) {
    if (!ENV_DRAFT_STORE) return;
    try {
        ENV_DRAFT_STORE.setItem(ENV_DRAFT_PREFIX + ctx, JSON.stringify(rows.map(function (r) {
            return { k: r.k || '', v: r.v == null ? '' : String(r.v) };
        })));
    } catch (e) { /* storage penuh / diblokir: abaikan */ }
}
function envDraftClear(ctx) {
    if (!ENV_DRAFT_STORE) return;
    try { ENV_DRAFT_STORE.removeItem(ENV_DRAFT_PREFIX + ctx); } catch (e) { }
}
function envHasContent(rows) {
    return rows.some(function (r) { return (r.k || '').trim() || (r.v || '').trim(); });
}
// Kalau state kosong tapi draft punya isi, pulihkan draft (dan beri tahu di console).
function envRecover(ctx, rows) {
    if (envHasContent(rows)) return rows;
    const saved = envDraftRead(ctx);
    if (saved && envHasContent(saved)) {
        console.warn('[env] state kosong, dipulihkan dari draft:', ctx);
        return saved;
    }
    return rows;
}

function deployEnvRowHtml(e, i) {
    return '<div class="flex gap-2" data-env-row>' +
        '<input type="text" class="input font-mono sm:w-44" pattern="[A-Za-z_][A-Za-z0-9_]*" ' +
        'id="deployEnvKey-' + i + '" name="deployEnvKey-' + i + '" ' +
        'aria-label="Environment variable key ' + (i + 1) + '" ' +
        'value="' + esc(e.k || '') + '" ' +
        'data-deploy-env-k="' + i + '" placeholder="KEY" list="envKeySuggestions" ' +
        'autocomplete="off" spellcheck="false">' +
        '<input type="text" class="input flex-1 font-mono" ' +
        'id="deployEnvValue-' + i + '" name="deployEnvValue-' + i + '" ' +
        'aria-label="Environment variable value ' + (i + 1) + '" ' +
        'value="' + esc(e.v || '') + '" ' +
        'data-deploy-env-v="' + i + '" placeholder="value" ' +
        'autocomplete="off" spellcheck="false">' +
        '<button type="button" class="icon-btn shrink-0 border border-line" ' +
        'data-deploy-env-del="' + i + '" aria-label="Remove variable ' + (i + 1) + '">' +
        svgIcon('x', 'h-4 w-4') + '</button>' +
        '</div>';
}

function renderDeployEnv() {
    const w = $('#deployEnvList');
    if (!deployEnvVars.length) {
        w.innerHTML = '<p class="rounded-lg border border-line bg-subtle px-3 py-2 text-xs text-zinc-500">No variables yet — click Add or upload a folder with <code class="rounded bg-panel px-1">.env</code>.</p>';
        return;
    }
    w.innerHTML = deployEnvVars.map(deployEnvRowHtml).join('');
}

// Tambah baris TANPA membangun ulang list, jadi input yang sudah diisi tidak ikut terhapus.
function appendDeployEnvRow() {
    const w0 = $('#deployEnvList');
    const nodesBefore = Array.from(w0.querySelectorAll('[data-env-row]'));

    syncDeployEnvFromDOM();
    deployEnvVars = envRecover('deploy', deployEnvVars);
    deployEnvVars.push({ k: '', v: '' });
    envDraftWrite('deploy', deployEnvVars);

    const w = $('#deployEnvList');
    const i = deployEnvVars.length - 1;
    if (!w.querySelector('[data-deploy-env-k]')) w.innerHTML = '';
    w.insertAdjacentHTML('beforeend', deployEnvRowHtml(deployEnvVars[i], i));

    const nodesAfter = Array.from(w.querySelectorAll('[data-env-row]'));
    console.log('[add] preserved rows:',
        nodesBefore.every(function (n, idx) { return n === nodesAfter[idx]; }),
        'before:', nodesBefore.length, 'after:', nodesAfter.length,
        'state:', JSON.parse(JSON.stringify(deployEnvVars)));

    const ks = $$('[data-deploy-env-k]', w);
    if (ks.length) ks[ks.length - 1].focus();
}

function parseEnvContent(text) {
    const result = [];
    const lines = text.split(/\r?\n/);
    let i = 0;

    while (i < lines.length) {
        let raw = lines[i];
        i++;

        // Hapus BOM di awal file
        if (i === 1) raw = raw.replace(/^\uFEFF/, '');

        let line = raw.trim();
        if (!line || line.startsWith('#')) continue;

        // Support `export KEY=VALUE`
        line = line.replace(/^export\s+/i, '');

        // Cari separator `=` pertama
        const eq = line.indexOf('=');
        if (eq < 1) continue;

        let key = line.slice(0, eq).trim();
        let value = line.slice(eq + 1).trim();

        // Validasi key — harus match pola yang backend terima
        if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(key)) continue;

        // Quote handling: kalau value diawali `"` atau `'`, kita baca sampai quote penutup
        // (bisa multi-line untuk value panjang)
        if (value.startsWith('"') || value.startsWith("'")) {
            const quote = value[0];
            // Kalau quote penutup ada di baris ini juga
            if (value.length > 1 && value.endsWith(quote) && value.length > 2) {
                value = value.slice(1, -1);
            } else {
                // Multi-line: gabung sampai ketemu quote penutup
                let acc = value.slice(1); // buang opening quote
                while (i < lines.length && !acc.endsWith(quote)) {
                    acc += '\n' + lines[i];
                    i++;
                }
                if (acc.endsWith(quote)) acc = acc.slice(0, -1);
                value = acc;
            }
        } else {
            // Tanpa quote — strip inline comment (` #...`)
            const hashIdx = value.indexOf(' #');
            if (hashIdx > -1) value = value.slice(0, hashIdx).trim();
            // Strip trailing comment dengan tab
            const tabIdx = value.indexOf('\t#');
            if (tabIdx > -1) value = value.slice(0, tabIdx).trim();
        }

        // Unescape escaped quotes/backslash
        value = value.replace(/\\"/g, '"').replace(/\\'/g, "'").replace(/\\n/g, '\n');

        result.push({ k: key, v: value });
    }
    return result;
}

// Read the shallowest .env-family file from the picked folder, without
// touching the ZIP contents (still filtered for security).
async function detectEnvFromFiles(files) {
    // Urutan prioritas: paling spesifik dulu. Depth paling dangkal menang
    // saat ada multiple file di folder berbeda.
    const PRIORITY = [
        '.env.local',
        '.env.production',
        '.env.development',
        '.env',
        'env.example',
        '.env.example',
    ];

    let best = null;
    let bestScore = -Infinity;

    for (let i = 0; i < files.length; i++) {
        const f = files[i];
        const idx = PRIORITY.indexOf(f.name);
        if (idx === -1) continue;

        const rel = f.webkitRelativePath || f.name;
        const depth = rel.split('/').length;

        // Skor lebih tinggi = lebih dipilih.
        // Prioritas utama: nama file (idx lebih kecil = lebih baik).
        // Tiebreaker: depth lebih kecil (lebih dangkal = lebih baik).
        const score = (PRIORITY.length - idx) * 1000 - depth;

        if (score > bestScore) {
            bestScore = score;
            best = f;
        }
    }

    if (!best) return [];

    try {
        const text = await best.text();
        const parsed = parseEnvContent(text);
        console.log('[env] auto-detected from', best.webkitRelativePath || best.name, '→', parsed.length, 'vars');
        return parsed;
    } catch (e) {
        console.warn('[env] failed to read', best.name, e);
        return [];
    }
}

// ─── Client-side ZIP packaging ─────────────────────────────────────────
// Filter path relatif root project, buang artefak build & dependency.
// Ini mengurangi ukuran upload dan mencegah buildpack salah deteksi.
const ZIP_SKIP_PATTERNS = [
    /(^|\/)node_modules(\/|$)/,
    /(^|\/)\.git(\/|$)/,
    /(^|\/)\.vite(\/|$)/,
    /(^|\/)\.next(\/|$)/,
    /(^|\/)\.nuxt(\/|$)/,
    /(^|\/)\.cache(\/|$)/,
    /(^|\/)coverage(\/|$)/,
    /(^|\/)__MACOSX(\/|$)/,
    /(^|\/)dist(\/|$)/,
    /(^|\/)build(\/|$)/,
    /(^|\/)out(\/|$)/,
    /(^|\/)\.DS_Store$/,
    /\.log$/,
];

function shouldSkipPath(p) {
    return ZIP_SKIP_PATTERNS.some(function (re) { return re.test(p); });
}

// detectCommonRoot: jika SEMUA file ada di bawah satu folder root
// (mis. "my-app/src/a.js" dan "my-app/package.json"), return "my-app/".
// Return "" jika tidak ada root yang sama.
function detectCommonRoot(paths) {
    if (paths.length === 0) return '';
    const firstSeg = paths[0].split('/')[0];
    if (!firstSeg || paths[0].indexOf('/') === -1) return '';
    const prefix = firstSeg + '/';
    for (let i = 0; i < paths.length; i++) {
        if (paths[i].indexOf(prefix) !== 0) return '';
    }
    return prefix;
}

// buildProjectZip membangun ZIP dengan path relatif dari root project.
// onProgress(done, total) dipanggil tiap file selesai dibaca.
async function buildProjectZip(files, onProgress) {
    if (!window.fflate || !window.fflate.zip) {
        throw new Error('ZIP library (fflate) belum dimuat');
    }
    if (!files || files.length === 0) {
        throw new Error('Folder kosong');
    }

    // 1. Filter
    const filtered = [];
    for (let i = 0; i < files.length; i++) {
        const f = files[i];
        const rel = f.webkitRelativePath || f.name;
        if (!rel) continue;
        if (shouldSkipPath(rel)) continue;
        filtered.push({ file: f, path: rel });
    }
    if (filtered.length === 0) {
        throw new Error('Tidak ada file tersisa setelah filter (semua ada di node_modules/.git/dist, dsb.)');
    }

    // 2. Strip common root folder agar paths di dalam ZIP bersih.
    const rootPrefix = detectCommonRoot(filtered.map(function (x) { return x.path; }));

    // 3. Baca semua file → Uint8Array
    const entries = {};
    let done = 0;
    for (let i = 0; i < filtered.length; i++) {
        const item = filtered[i];
        const key = rootPrefix ? item.path.slice(rootPrefix.length) : item.path;
        try {
            const buf = await item.file.arrayBuffer();
            entries[key] = new Uint8Array(buf);
        } catch (e) {
            throw new Error('Gagal membaca "' + key + '": ' + e.message);
        }
        done++;
        if (onProgress) onProgress(done, filtered.length);
    }

    // 4. Build ZIP
    return await new Promise(function (resolve, reject) {
        window.fflate.zip(entries, { level: 6 }, function (err, data) {
            if (err) return reject(new Error('ZIP gagal: ' + err.message));
            resolve(data); // Uint8Array
        });
    });
}

// ─── Deploy Progress Panel ────────────────────────────────────────────────────
// State
let _dp = {
    ws: null,           // active WebSocket untuk app yang sedang di-deploy
    appId: null,        // ID app yang sedang berjalan
    depId: null,        // ID deployment
    appSlug: null,      // slug app (untuk WS room)
    startTs: null,      // timestamp mulai (ms)
    elapsedTimer: null, // setInterval untuk counter
    autoDismiss: null,  // setTimeout dismiss otomatis setelah sukses
    pollTimer: null,    // fallback interval polling
    isRunning: false,   // true selama pipeline deploy masih berjalan
    lastIdx: 0,         // indeks tahap terakhir
};

// Definisi 4 tahapan pipeline
const DP_STAGES = [
    { id: 'source', label: 'Source', desc: 'Preparing source code' },
    { id: 'building', label: 'Build', desc: 'Building container image' },
    { id: 'deploying', label: 'Deploy', desc: 'Starting container' },
    { id: 'running', label: 'Live', desc: 'Ready to launch' },
];

// ── Stepper renderer ──────────────────────────────────────────────────────────
// state: 'waiting' | 'active' | 'done' | 'failed'
function _dpStepState(stageIdx, activeIdx, failed, finished) {
    if (failed && stageIdx === activeIdx) return 'failed';
    if (finished) return 'done';
    if (stageIdx < activeIdx) return 'done';
    if (stageIdx === activeIdx) return 'active';
    return 'waiting';
}

function _dpRenderSteps(activeIdx, failed, finished) {
    _dp.lastIdx = activeIdx;
    const ol = $('#dpSteps');
    ol.innerHTML = DP_STAGES.map(function (s, i) {
        const st = _dpStepState(i, activeIdx, !!failed, !!finished);
        const isLast = i === DP_STAGES.length - 1;

        // Icon
        let iconHTML;
        if (st === 'done') {
            iconHTML = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" class="h-3.5 w-3.5" aria-hidden="true"><path d="M20 6 9 17l-5-5"/></svg>';
        } else if (st === 'active') {
            iconHTML = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="h-3.5 w-3.5 dp-spinning" aria-hidden="true"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>';
        } else if (st === 'failed') {
            iconHTML = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-3.5 w-3.5" aria-hidden="true"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>';
        } else {
            iconHTML = '<span class="text-[10px] font-bold tabular-nums text-zinc-500">' + (i + 1) + '</span>';
        }

        const iconBg =
            st === 'done' ? 'bg-ok border-ok text-white' :
                st === 'active' ? 'bg-accent border-accent text-white' :
                    st === 'failed' ? 'bg-danger border-danger text-white' :
                        'bg-subtle border-line text-zinc-500';

        // Connector line
        const lineColor = (st === 'done' || finished || (i < activeIdx)) ? 'bg-ok' : 'bg-line';

        return '<li class="flex flex-1 flex-col items-center ' + (isLast ? '' : 'pr-2') + '">' +
            // Row: icon + line
            '<div class="flex w-full items-center">' +
            '<div class="dp-step-icon flex h-7 w-7 shrink-0 items-center justify-center rounded-full border ' + iconBg + '">' +
            iconHTML +
            '</div>' +
            (!isLast ? '<div class="dp-step-line mx-1 h-0.5 flex-1 ' + lineColor + '"></div>' : '') +
            '</div>' +
            // Label below
            '<div class="mt-1.5 w-full pl-0.5 pr-1">' +
            '<p class="text-[11px] font-medium ' + (st === 'active' ? 'text-accent' : st === 'done' ? 'text-ok' : st === 'failed' ? 'text-danger' : 'text-zinc-500') + '">' + s.label + '</p>' +
            '<p class="text-[10px] text-zinc-600 leading-tight">' + s.desc + '</p>' +
            '</div>' +
            '</li>';
    }).join('');
}

// ── Stage index helpers ────────────────────────────────────────────────────────
function _dpStageIndex(status) {
    if (!status) return _dp.lastIdx !== undefined ? _dp.lastIdx : 1;
    const s = status.toLowerCase();
    if (s === 'source' || s === 'fetch' || s === 'upload' || s === 'uploading' || s === 'pending') return 0;
    if (s === 'building') return 1;
    if (s === 'deploying') return 2;
    if (s === 'running' || s === 'live' || s === 'done') return 3;
    return _dp.lastIdx !== undefined ? _dp.lastIdx : 1;
}

// ── Show / hide helpers ────────────────────────────────────────────────────────
function dpShow() {
    const el = $('#deployProgress');
    el.classList.remove('hidden');
    requestAnimationFrame(function () {
        requestAnimationFrame(function () {
            el.classList.remove('dp-hidden');
        });
    });
    // Switch to overview if not already there
    if (current !== 'overview') navigate('overview');
}

function dpHideUI() {
    const el = $('#deployProgress');
    el.classList.add('dp-hidden');
    setTimeout(function () { el.classList.add('hidden'); }, 280);
}

function dpHide() {
    dpHideUI();
    // Jangan putus tracking jika masih running di background
    if (!_dp.isRunning) {
        _dpCleanup();
    }
}

function _dpCleanup() {
    if (_dp.ws) { try { _dp.ws.close(); } catch (_) { } _dp.ws = null; }
    if (_dp.elapsedTimer) { clearInterval(_dp.elapsedTimer); _dp.elapsedTimer = null; }
    if (_dp.autoDismiss) { clearTimeout(_dp.autoDismiss); _dp.autoDismiss = null; }
    if (_dp.pollTimer) { clearInterval(_dp.pollTimer); _dp.pollTimer = null; }
    _dp.isRunning = false;
    _dp.appId = null;
    _dp.depId = null;
    _dp.appSlug = null;
    _dp.lastIdx = 0;
}

// ── Elapsed counter ───────────────────────────────────────────────────────────
function _dpStartElapsed() {
    _dp.startTs = Date.now();
    _dp.elapsedTimer = setInterval(function () {
        const sec = Math.floor((Date.now() - _dp.startTs) / 1000);
        const m = Math.floor(sec / 60);
        const s = sec % 60;
        const t = m > 0 ? m + 'm ' + s + 's' : s + 's';
        $('#dpElapsed').textContent = t;
    }, 1000);
}

// ── Log appender ─────────────────────────────────────────────────────────────
function _dpAppendLog(line) {
    const pre = $('#dpLog');
    const span = document.createElement('span');
    span.className = 'dp-log-line block';
    span.textContent = line;
    pre.appendChild(span);
    // Keep last 120 lines
    while (pre.children.length > 120) pre.removeChild(pre.firstChild);
    pre.scrollTop = pre.scrollHeight;
}

// ── Final handlers (Success / Failure) ────────────────────────────────────────
function _dpOnSuccess(url, duration) {
    if (!_dp.isRunning) return;
    _dp.isRunning = false;
    if (_dp.pollTimer) { clearInterval(_dp.pollTimer); _dp.pollTimer = null; }
    if (_dp.ws) { try { _dp.ws.close(); } catch (_) { } _dp.ws = null; }

    _dpRenderSteps(3, false, true); // Semua step centang hijau ✅
    $('#dpTitle').textContent = _dp.appSlug + ' is live!';
    $('#dpSub').textContent = url ? '→ ' + url : 'Deployment complete';

    const hi = $('#dpHeaderIcon');
    hi.classList.remove('dp-spinning');
    hi.outerHTML = '<svg id="dpHeaderIcon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4 text-ok" aria-hidden="true"><path d="M21.801 10A10 10 0 1 1 17 3.335"/><path d="m9 11 3 3L22 4"/></svg>';

    if (_dp.elapsedTimer) { clearInterval(_dp.elapsedTimer); _dp.elapsedTimer = null; }
    _dpAppendLog('── Deployment complete' + (duration ? ' in ' + duration : '') + ' ──');

    toast('success', 'Deployment complete', (_dp.appSlug || 'App') + ' is now running.');
    fetchApps();
    fetchAllDeploys();

    // Auto-dismiss setelah 8 detik
    _dp.autoDismiss = setTimeout(function () { dpHideUI(); _dpCleanup(); }, 8000);
}

function _dpOnFailure(errorMsg, status) {
    if (!_dp.isRunning) return;
    _dp.isRunning = false;
    if (_dp.pollTimer) { clearInterval(_dp.pollTimer); _dp.pollTimer = null; }
    if (_dp.ws) { try { _dp.ws.close(); } catch (_) { } _dp.ws = null; }

    const curIdx = _dpStageIndex(status || 'building');
    _dpRenderSteps(curIdx, true, false); // Step yang gagal jadi silang merah ❌
    $('#dpTitle').textContent = 'Deploy failed';
    $('#dpSub').textContent = errorMsg || 'An error occurred';

    const hi = $('#dpHeaderIcon');
    hi.outerHTML = '<svg id="dpHeaderIcon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4 text-danger" aria-hidden="true"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/></svg>';

    if (_dp.elapsedTimer) { clearInterval(_dp.elapsedTimer); _dp.elapsedTimer = null; }
    _dpAppendLog('── FAILED: ' + (errorMsg || 'unknown error') + ' ──');

    toast('danger', 'Deploy failed', errorMsg || _dp.appSlug);
    fetchApps();
    fetchAllDeploys();
}

// ── Fallback Polling ──────────────────────────────────────────────────────────
function _dpStartPoll(appId, depId) {
    if (_dp.pollTimer) clearInterval(_dp.pollTimer);
    _dp.pollTimer = setInterval(async function () {
        if (!_dp.isRunning) {
            clearInterval(_dp.pollTimer);
            _dp.pollTimer = null;
            return;
        }
        try {
            const res = await apiFetch('/api/apps/' + appId + '/deployments');
            const list = Array.isArray(res) ? res : (res && Array.isArray(res.deployments) ? res.deployments : []);
            if (!list.length) return;
            const target = depId ? list.find(function (d) { return d.id === depId; }) : list[0];
            if (!target) return;
            const st = (target.status || '').toLowerCase();
            if (st === 'running' || st === 'success') {
                _dpOnSuccess(target.url, target.duration);
            } else if (st === 'failed') {
                _dpOnFailure(target.error || target.last_err || 'Deploy failed', 'building');
            } else if (st === 'building' || st === 'deploying') {
                const idx = _dpStageIndex(st);
                _dpRenderSteps(idx, false, false);
                $('#dpSub').textContent = DP_STAGES[idx].desc;
            }
        } catch (_) { /* continue polling */ }
    }, 3000);
}

// ── Main: start progress tracking for a deployed app ─────────────────────────
function dpStart(appId, appSlug, deploymentId, sourceType) {
    _dpCleanup();
    _dp.appId = appId;
    _dp.depId = deploymentId;
    _dp.appSlug = appSlug;
    _dp.isRunning = true;

    let initialSub = 'Preparing source code…';
    if (sourceType === 'upload') initialSub = 'Uploading folder to server…';
    else if (sourceType === 'github') initialSub = 'Cloning GitHub repository…';
    else if (sourceType === 'local') initialSub = 'Reading server local folder…';

    $('#dpLog').textContent = '';
    $('#dpElapsed').textContent = '';
    $('#dpTitle').textContent = 'Deploying ' + appSlug + '…';
    $('#dpSub').textContent = initialSub;

    const headerIcon = $('#dpHeaderIcon');
    if (headerIcon) {
        // ✅ SVG: pakai setAttribute, BUKAN .className
        headerIcon.setAttribute('class', 'h-4 w-4 text-accent dp-spinning');
        headerIcon.setAttribute('stroke', 'currentColor');
    }

    _dpRenderSteps(0, false, false);
    dpShow();
    _dpStartElapsed();

    // WebSocket: connect to app log stream
    const wsPath = '/api/ws/apps/' + appId;
    const ws = new WebSocket(wsURL(wsPath));
    _dp.ws = ws;

    ws.onopen = function () {
        _dpAppendLog('── Stream connected ──');
    };

    ws.onmessage = function (ev) {
        let msg;
        try { msg = JSON.parse(ev.data); } catch (_) {
            _dpAppendLog(ev.data);
            return;
        }

        // Handle typed events from worker
        if (msg.type === 'log' && msg.line) {
            _dpAppendLog(msg.line);
        } else if (msg.type === 'status') {
            const status = (msg.status || '').toLowerCase();
            const idx = _dpStageIndex(status);
            _dpRenderSteps(idx, false, false);
            $('#dpSub').textContent = DP_STAGES[Math.min(idx, DP_STAGES.length - 1)].desc;

            if (status === 'building') {
                $('#dpTitle').textContent = 'Building ' + appSlug + '…';
                $('#dpSub').textContent = 'Building container image with buildpacks';
            } else if (status === 'deploying') {
                $('#dpTitle').textContent = 'Analyzing configuration…';
                $('#dpSub').textContent = 'Starting container and health check';
            }
        } else if (msg.type === 'config_resolved') {
            const pt = msg.project_type || 'unknown';
            const src = msg.config_source || 'auto';
            const srcLabel = ({
                'wdeploy.json': 'wdeploy.json',
                'fe': 'pilihan UI',
                'auto': 'auto-detect',
                'default': 'default',
            })[src] || src;

            _dpAppendLog('[config] ' + pt + ' (' + srcLabel + ')');

            if (_dp.lastIdx === 0) {
                $('#dpTitle').textContent = 'Deploying ' + appSlug + '…';
                $('#dpSub').textContent = 'Kategori: ' + pt + ' · ' + srcLabel;
            }
        } else if (msg.type === 'done') {
            _dpOnSuccess(msg.url, msg.duration);
        } else if (msg.type === 'error') {
            _dpOnFailure(msg.error, msg.status);
        }
    };

    ws.onerror = function () {
        _dpAppendLog('[WebSocket error – fallback to polling]');
        _dpStartPoll(appId, deploymentId);
    };

    ws.onclose = function () {
        if (_dp.ws === ws) _dp.ws = null;
        if (_dp.isRunning) {
            _dpStartPoll(appId, deploymentId);
        }
    };
}

// Bind dismiss button
document.addEventListener('DOMContentLoaded', function () {
    $('#btnDpDismiss').addEventListener('click', dpHide);
    $('#btnDpViewFull').addEventListener('click', function () {
        if (!_dp.appId) return;
        const c = CONTAINERS.find(function (x) { return String(x.id) === String(_dp.appId); });
        if (c) { containerAction('logs', _dp.appId); }
    });
});

// ─── Deploy (POST /api/deploy) ────────────────────────────────────────────────
async function deployNow() {
    if ($('#btnDeploy').disabled) return;

    const isImp = deployTab === 'import';
    const repoVal = $('#selRepo').value;
    const branch = $('#inpBranch').value.trim() || 'main';

    const localPath = $('#inpLocalPath').value.trim();
    const localFiles = $('#inpLocalFiles').files;
    const isUpload = localFiles.length > 0;
    const projectType = ($('#selProjectType')?.value || '').trim();

    // Validasi
    if (isImp && !repoVal) {
        toast('info', 'Repository required', 'Choose a repository to deploy from.');
        return;
    }
    if (!isImp && !isUpload && !localPath) {
        toast('info', 'Source required', 'Please upload a folder or provide a server path.');
        return;
    }

    $('#btnDeploy').disabled = true;
    $('#btnDeployCancel').disabled = true;
    $('#btnDeployX').disabled = true;
    $('#deploySpin').classList.remove('hidden');
    $('#deployLabel').textContent = isUpload ? 'Uploading...' : 'Deploying…';

    // Collect env vars (skip empty keys)
    const envMap = {};
    let invalidEnv = null;
    syncDeployEnvFromDOM();
    deployEnvVars.forEach(function (e) {
        const k = (e.k || '').trim();
        if (!k) return;
        if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(k)) {
            if (!invalidEnv) invalidEnv = k;
            return;
        }
        envMap[k] = e.v;
    });
    if (invalidEnv) {
        toast('danger', 'Invalid env key', '"' + invalidEnv + '" — hanya A-Z, a-z, 0-9, _. Tidak boleh mulai angka.');
        $('#btnDeploy').disabled = false;
        $('#btnDeployCancel').disabled = false;
        $('#btnDeployX').disabled = false;
        $('#deploySpin').classList.add('hidden');
        $('#deployLabel').textContent = 'Deploy';
        return;
    }
    const dbEngine = $('#selDatabase').value;
    if (dbEngine && envMap.DATABASE_URL) {
        toast('info', 'DATABASE_URL ignored', 'A managed database is selected — DATABASE_URL will be set automatically.');
        delete envMap.DATABASE_URL;
    }

    let payload;
    let headers = {};

    if (isImp) {
        // 1. GitHub
        payload = { source_type: 'github', repo_full_name: repoVal, branch: branch };
        if (projectType) payload.project_type = projectType;
        if (dbEngine) payload.database = dbEngine;
        if (Object.keys(envMap).length) payload.env = envMap;
        headers['Content-Type'] = 'application/json';
    } else if (isUpload) {
        // 2. Upload Folder → ZIP
        $('#deployLabel').textContent = 'Packaging…';
        try {
            const zipBytes = await buildProjectZip(localFiles, function (done, total) {
                $('#deployLabel').textContent = 'Packaging ' + done + '/' + total;
            });

            const folderName = localFiles[0].webkitRelativePath.split('/')[0] || 'app';
            const slug = folderName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'uploaded-app';

            payload = new FormData();
            payload.append('source', 'upload');
            if (projectType) payload.append('project_type', projectType);
            if (dbEngine) payload.append('database', dbEngine);
            if (Object.keys(envMap).length) payload.append('env', JSON.stringify(envMap));
            payload.append('name', folderName);
            payload.append('slug', slug);
            payload.append('archive', new Blob([zipBytes], { type: 'application/zip' }), slug + '.zip');
        } catch (e) {
            console.error('[deploy] packaging failed:', e);
            toast('danger', 'Packaging failed', e.message);
            $('#btnDeploy').disabled = false;
            $('#btnDeployCancel').disabled = false;
            $('#btnDeployX').disabled = false;
            $('#deploySpin').classList.add('hidden');
            $('#deployLabel').textContent = 'Deploy';
            return;
        }
    } else {
        // 3. Server Absolute Path
        payload = { source: 'local', path: localPath };
        if (projectType) payload.project_type = projectType;
        if (dbEngine) payload.database = dbEngine;
        if (Object.keys(envMap).length) payload.env = envMap;
        headers['Content-Type'] = 'application/json';
    }

    try {
        // Panggil apiFetch. Perhatikan jika isUpload, kita tidak perlu JSON.stringify
        const deployResp = await apiFetch('/api/deploy', {
            method: 'POST',
            body: payload,
            headers: headers
        });

        envDraftClear('deploy');
        closeModal('modalDeploy');
        toast('success', 'Deployment queued', (isImp ? repoVal + '@' + branch : (isUpload ? 'Uploaded files' : localPath)) + ' · building…');

        if (deployResp && deployResp.app && deployResp.app.id) {
            const slug = deployResp.app.slug || deployResp.app.name || 'app';
            const depId = deployResp.deployment_id;
            const sourceType = isImp ? 'github' : (isUpload ? 'upload' : 'local');

            try {
                dpStart(deployResp.app.id, slug, depId, sourceType);
            } catch (uiErr) {
                // Deploy SUDAH sukses di server — hanya panel UI yang gagal
                console.error('[deploy] progress panel error (deploy tetap sukses):', uiErr);
            }
        }

        await fetchApps();
        await fetchAllDeploys();
    } catch (err) {
        console.error('[deploy] error:', err);
        toast('danger', 'Deploy failed', err.message);
        $('#btnDeploy').disabled = false;
        $('#btnDeployCancel').disabled = false;
        $('#btnDeployX').disabled = false;
        $('#deploySpin').classList.add('hidden');
        $('#deployLabel').textContent = 'Deploy';
    }
}

// ─── Env Vars (/api/apps/{id}/env) ───────────────────────────────────────────

async function fetchEnvVars(appId) {
    try {
        const data = await apiFetch('/api/apps/' + appId + '/env');
        envVars = normalizeEnvResponse(data);
        currentAppId = appId;
        renderEnvVars();
    } catch (err) {
        toast('danger', 'Env error', err.message);
    }
}

async function saveEnvVars(appId) {
    try {
        const envObj = {};
        envVars.forEach(function (e) {
            const k = (e.k || '').trim();
            if (!k) return;
            if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(k)) return;
            // Pastikan value scalar
            if (e.v !== null && typeof e.v === 'object') return;
            envObj[k] = String(e.v ?? '');
        });
        const payload = { env: envObj };

        await apiFetch('/api/apps/' + appId + '/env', { method: 'PUT', body: payload });
        toast('success', 'Env vars saved', 'Environment updated for ' + appId + '.');
        envDraftClear(settingsEnvCtx());
        dirty = false;
        $('#btnSave').disabled = true;
    } catch (err) {
        toast('danger', 'Save failed', err.message);
    }
}

async function deleteEnvVar(appId, key) {
    try {
        await apiFetch('/api/apps/' + appId + '/env/' + encodeURIComponent(key), { method: 'DELETE' });
        toast('success', 'Variable removed', key + ' deleted.');
        // Hapus hanya baris itu; jangan refetch supaya baris lain yang belum disimpan tidak hilang.
        envVars = envVars.filter(function (e) { return e.k !== key; });
        envDraftWrite(settingsEnvCtx(), envVars);
        renderEnvVars();
    } catch (err) {
        toast('danger', 'Delete failed', err.message);
    }
}

// ─── WebSocket log stream (/api/ws/apps/{id}) ────────────────────────────────

let currentLogWs = null;

function openLogStream(appId) {
    if (currentLogWs) { try { currentLogWs.close(); } catch (_) { } currentLogWs = null; }
    const ws = new WebSocket(wsURL('/api/ws/apps/' + appId));   // ← path relatif
    currentLogWs = ws;

    const logBody = $('#logsBody');
    logBody.textContent = 'Connecting to log stream…\n';

    ws.onopen = function () {
        logBody.textContent = '── Stream connected ──\n';
    };
    ws.onmessage = function (ev) {
        let line = ev.data;
        try {
            const msg = JSON.parse(ev.data);
            if (msg.type === 'log' && msg.line !== undefined) {
                line = msg.line;
            } else if (msg.type === 'status') {
                line = '── Status: ' + msg.status + ' ──';
            } else if (msg.type === 'config_resolved') {
                line = '── Config: type=' + (msg.project_type || '?') +
                    ' (source=' + (msg.config_source || '?') + ') ──';
            } else if (msg.type === 'done') {
                line = '── Deployment live: ' + (msg.url || '') + ' ──';
            } else if (msg.type === 'error') {
                line = '── Error: ' + (msg.error || '') + ' ──';
            }
        } catch (_) { }
        logBody.textContent += line + '\n';
        logBody.scrollTop = logBody.scrollHeight;
    };
    ws.onerror = function () {
        logBody.textContent += '\n[WebSocket error – connection lost]';
    };
    ws.onclose = function () {
        logBody.textContent += '\n── Stream closed ──';
    };
}

function closeLogStream() {
    if (currentLogWs) { try { currentLogWs.close(); } catch (_) { } currentLogWs = null; }
}
// ─── Container stats WebSocket ──────────────────────────────────────
let _containersStatsWs = null;

function makeContainerStatsWS() {
    let ws = null, retryDelay = 2000, retryTimer = null, stopped = false;

    function connect() {
        if (stopped) return;
        ws = new WebSocket(wsURL('/api/ws/containers/stats'));

        ws.onopen = function () {
            console.log('[ws] container stats OPEN');
            retryDelay = 2000;
        };

        ws.onmessage = function (ev) {
            let msg;
            try { msg = JSON.parse(ev.data); } catch (_) { return; }
            if (msg.type !== 'stats' || !msg.containers) return;

            Object.keys(msg.containers).forEach(function (appId) {
                const s = msg.containers[appId];
                const c = CONTAINERS.find(function (x) { return String(x.id) === String(appId); });
                if (!c) return;

                c.cpu = (typeof s.cpu === 'number')
                    ? s.cpu.toFixed(1) + '%'
                    : '—';

                if (typeof s.memory_mb === 'number' && s.memory_mb > 0) {
                    c.mem = s.memory_mb >= 1024
                        ? (s.memory_mb / 1024).toFixed(1) + ' GB'
                        : s.memory_mb.toFixed(0) + ' MB';
                } else {
                    c.mem = '0 MB';
                }

                if (typeof s.uptime_seconds === 'number' && s.uptime_seconds > 0) {
                    c.uptime = formatUptime(s.uptime_seconds);
                } else {
                    c.uptime = '—';
                }
            });

            scheduleContainerRerender();
        };

        ws.onerror = function () { try { ws.close(); } catch (_) { } };

        ws.onclose = function () {
            if (stopped) return;
            console.log('[ws] container stats CLOSE — retry in', retryDelay, 'ms');
            retryTimer = setTimeout(connect, retryDelay);
            retryDelay = Math.min(retryDelay * 2, 30000);
        };
    }

    connect();
    return { close: function () { stopped = true; if (retryTimer) clearTimeout(retryTimer); if (ws) try { ws.close(); } catch (_) { } } };
}

function startContainerStatsMonitoring() {
    if (_containersStatsWs) return;
    _containersStatsWs = makeContainerStatsWS();
}

function stopContainerStatsMonitoring() {
    if (_containersStatsWs) { _containersStatsWs.close(); _containersStatsWs = null; }
}

// Batasi re-render: 20 update dalam 1 tick → 1 render
let _containerRerenderTimer = null;
function scheduleContainerRerender() {
    if (_containerRerenderTimer) return;
    _containerRerenderTimer = requestAnimationFrame(function () {
        _containerRerenderTimer = null;
        if (current === 'containers' || current === 'overview') renderRoute();
    });
}

function formatUptime(sec) {
    const d = Math.floor(sec / 86400);
    const h = Math.floor((sec % 86400) / 3600);
    const m = Math.floor((sec % 3600) / 60);
    if (d > 0) return d + 'd ' + h + 'h';
    if (h > 0) return h + 'h ' + m + 'm';
    if (m > 0) return m + 'm';
    return Math.floor(sec) + 's';
}
// ─── App state (non-data) — declared above with other let bindings ────────────

// Status badge builders
const CONT_STATUS = {
    Running: ['text-emerald-400', 'bg-emerald-400'],
    Stopped: ['text-zinc-400', 'bg-zinc-500'],
    Restarting: ['text-amber-400', 'bg-amber-400'],
    Building: ['text-amber-400', 'bg-amber-400'],
    Deploying: ['text-blue-400', 'bg-blue-400'],
    Failed: ['text-red-400', 'bg-red-400']
};
const DEPLOY_STATUS = {
    Success: ['text-emerald-400', 'bg-emerald-400'],
    Running: ['text-emerald-400', 'bg-emerald-400'],
    Failed: ['text-red-400', 'bg-red-400'],
    Building: ['text-amber-400', 'bg-amber-400'],
    Deploying: ['text-blue-400', 'bg-blue-400'],
    Canceled: ['text-zinc-400', 'bg-zinc-500']
};
function contBadge(s) {
    const m = CONT_STATUS[s] || CONT_STATUS.Stopped;
    return '<span class="badge ' + m[0] + '"><span class="badge-dot ' + m[1] + '"></span>' + s + '</span>';
}
function deployBadge(s) {
    const m = DEPLOY_STATUS[s] || DEPLOY_STATUS.Canceled;
    const pulse = s === 'Building' ? ' animate-pulse' : '';
    return '<span class="badge ' + m[0] + '"><span class="badge-dot ' + m[1] + pulse + '"></span>' + s + '</span>';
}
function providerChip(p) {
    if (p !== 'github' && p !== 'gitlab') return '<span class="text-xs text-zinc-500">—</span>';
    return '<span class="badge gap-1 px-1.5 text-zinc-400">' + providerLogo(p, 'h-3 w-3') + providerName(p) + '</span>';
}

// Container table rows (shared by Overview + Containers)
function renderContainerRows(tbody, list, withImage) {
    if (!list.length) {
        tbody.innerHTML = '<tr><td colspan="' + (withImage ? 7 : 6) + '" class="px-4 py-10 text-center text-xs text-zinc-500">No containers found.</td></tr>';
        return;
    }
    tbody.innerHTML = list.map(function (c) {
        const run = c.status === 'Running';
        // Tombol redeploy hanya ditampilkan untuk project upload/zip/local.
        // Project dari Git (GitHub/GitLab) otomatis di-deploy via webhook saat push ke branch main.
        const isZipOrLocal = c.sourceType === 'zip' || c.sourceType === 'local';
        const acts = [
            ['logs', 'Logs', false],
            ['edit-env', 'Env', false],       // ← new, works for all source types
        ];
        if (isZipOrLocal) {
            acts.push(['redeploy', 'Redeploy', false]);
        }
        acts.push(['delete', 'Delete', false]);
        const nameHtml = c.url
            ? '<a href="' + esc(c.url) + '" target="_blank" rel="noopener noreferrer" class="group/link inline-flex items-center gap-1.5 font-medium text-blue-400 hover:text-blue-300 hover:underline transition-colors" title="Buka ' + esc(c.url) + '">' +
            esc(c.service) +
            '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-3 w-3 shrink-0 opacity-80 group-hover/link:opacity-100 group-hover/link:translate-x-0.5 group-hover/link:-translate-y-0.5 transition-transform"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/></svg>' +
            '</a>'
            : '<p class="font-medium text-zinc-100">' + esc(c.service) + '</p>';

        return '<tr class="group h-12 border-b border-line transition-colors last:border-b-0 hover:bg-white/[0.02]">' +
            '<td class="whitespace-nowrap px-4 py-2">' + nameHtml + '<p class="mt-0.5 text-xs text-zinc-500">' + esc(c.id) + ' · ' + esc(c.region) + '</p></td>' +
            (withImage ? '<td class="whitespace-nowrap px-4 font-mono text-xs text-zinc-400">' + esc(c.image) + '</td>' : '') +
            '<td class="px-4 py-2">' + contBadge(c.status) + '</td>' +
            '<td class="px-4 py-2 tabular-nums">' + (run ? c.cpu : '—') + '</td>' +
            '<td class="px-4 py-2 tabular-nums">' + (run ? c.mem : '—') + '</td>' +
            '<td class="whitespace-nowrap px-4 py-2 tabular-nums">' + (run ? c.uptime : '—') + '</td>' +
            '<td class="px-4 py-2 text-right"><div class="flex justify-end gap-1.5 opacity-60 transition-opacity focus-within:opacity-100 group-hover:opacity-100">' +
            acts.map(function (a) {
                const danger = a[0] === 'delete' ? ' text-red-400 hover:bg-red-500/10' : '';
                return '<button type="button" data-act="' + a[0] + '" data-id="' + esc(c.id) + '" class="btn-xs' + danger + '">' + a[1] + '</button>';
            }).join('') +
            '</div></td></tr>';
    }).join('');
}

// (Log streaming is handled by openLogStream via WebSocket — see API layer above)

// Container actions (real API)
function containerAction(act, id) {
    const c = CONTAINERS.find(function (x) { return String(x.id) === String(id); });
    if (!c) return;
    if (act === 'logs') {
        $('#logsSub').textContent = c.id + ' · ' + c.image;
        $('#logsBody').textContent = 'Connecting…';
        openModal('modalLogs');
        openLogStream(id);
        return;
    }
    if (act === 'redeploy') {
        redeployApp(id);
        return;
    }
    if (act === 'edit-env') {
        openEditEnv(id);
        return;
    }
    if (act === 'delete') {
        if (!confirm('Delete ' + c.service + ' (' + id + ')? This cannot be undone.')) return;
        deleteApp(id);
        return;
    }
}
// ─── Edit Env + Redeploy ────────────────────────────────────────────
let _editEnvAppId = null;
let _editEnvVars = [];
let _editEnvBusy = false;
let _editEnvLoading = false;
let _editEnvReqId = 0;

async function openEditEnv(appId) {
    const c = CONTAINERS.find(function (x) { return String(x.id) === String(appId); });
    if (!c) return;

    const reqId = ++_editEnvReqId;
    _editEnvAppId = appId;
    _editEnvVars = [];
    envDraftClear('edit:' + appId);   // sumber awal = server, draft lama dibuang
    _editEnvLoading = true;
    $('#editEnvSub').textContent = c.service + ' · ' + c.id;
    $('#editEnvList').innerHTML = '<p class="text-xs text-zinc-500 py-2">Loading…</p>';
    $('#btnEditEnvSave').disabled = true;
    $('#btnEditEnvCancel').disabled = false;
    $('#editEnvSpin').classList.add('hidden');
    $('#editEnvLabel').textContent = 'Save & redeploy';
    openModal('modalEditEnv');

    try {
        const data = await apiFetch('/api/apps/' + appId + '/env');
        if (reqId !== _editEnvReqId) return;
        _editEnvVars = normalizeEnvResponse(data);
        if (!_editEnvVars.length) _editEnvVars = [{ k: '', v: '' }];
    } catch (e) {
        if (reqId !== _editEnvReqId) return;
        toast('danger', 'Env load failed', e.message);
        _editEnvVars = [{ k: '', v: '' }];
    }
    if (reqId !== _editEnvReqId) return;
    _editEnvLoading = false;
    renderEditEnv();
    $('#btnEditEnvSave').disabled = _editEnvBusy;
    $('#btnEditEnvCancel').disabled = false;
}

function normalizeEnvResponse(data) {
    if (!data) return [];

    // Case A: array [{key, value}] atau [{k, v}]
    if (Array.isArray(data)) {
        return data
            .map(function (e) {
                return {
                    k: String(e.key || e.k || '').trim(),
                    v: e.value !== undefined ? String(e.value) : String(e.v ?? ''),
                };
            })
            .filter(function (e) { return e.k && /^[A-Za-z_][A-Za-z0-9_]*$/.test(e.k); });
    }

    // Case B: wrapper { env: {...}, ... } — PALING UMUM di backend modern.
    // Kalau ada field `env` berupa object, pakai itu SAJA. Metadata lain diabaikan.
    if (data.env && typeof data.env === 'object' && !Array.isArray(data.env)) {
        return Object.entries(data.env)
            .map(function (p) {
                return { k: String(p[0]).trim(), v: String(p[1] ?? '') };
            })
            .filter(function (e) { return e.k && /^[A-Za-z_][A-Za-z0-9_]*$/.test(e.k); });
    }

    // Case C: flat map {KEY: VALUE} — filter metadata yang suka nyelip.
    // Aturan:
    //   1. Key harus match env var naming pattern.
    //   2. Value HARUS scalar (string/number/boolean/null) — array/object = metadata.
    //   3. Key lowercase satu kata yang biasanya metadata (node, npm, go, ...) → skip.
    const METADATA_KEYS = new Set([
        'node', 'npm', 'yarn', 'pnpm', 'bun',
        'go', 'python', 'ruby', 'java', 'rust', 'php',
        'env', 'warnings', 'errors', 'project', 'project_type',
        'buildpack', 'database', 'source', 'status', 'message',
        'created_at', 'updated_at', 'app_id', 'id',
        'slug', 'name', 'port', 'branch', 'repo', 'version',
        'url', 'region', 'image', 'memory', 'cpu', 'uptime',
    ]);

    return Object.entries(data)
        .filter(function (p) {
            const [k, v] = p;

            // Rule 1: valid env var key
            if (!k || !/^[A-Za-z_][A-Za-z0-9_]*$/.test(k)) return false;

            // Rule 2: value harus scalar
            if (v !== null && typeof v === 'object') return false;

            // Rule 3: blocklist metadata
            if (METADATA_KEYS.has(k.toLowerCase())) return false;

            return true;
        })
        .map(function (p) {
            return { k: String(p[0]).trim(), v: String(p[1] ?? '') };
        });
}

function editEnvRowHtml(e, i) {
    return '<div class="flex gap-2" data-env-row>' +
        '<input type="text" class="input font-mono sm:w-44" pattern="[A-Za-z_][A-Za-z0-9_]*" ' +
        'id="editEnvKey-' + i + '" name="editEnvKey-' + i + '" ' +
        'aria-label="Environment variable key ' + (i + 1) + '" ' +
        'value="' + esc(e.k || '') + '" ' +
        'data-edit-env-k="' + i + '" placeholder="KEY" list="envKeySuggestions" ' +
        'autocomplete="off" spellcheck="false">' +
        '<input type="text" class="input flex-1 font-mono" ' +
        'id="editEnvValue-' + i + '" name="editEnvValue-' + i + '" ' +
        'aria-label="Environment variable value ' + (i + 1) + '" ' +
        'value="' + esc(e.v || '') + '" ' +
        'data-edit-env-v="' + i + '" placeholder="value" ' +
        'autocomplete="off" spellcheck="false">' +
        '<button type="button" class="icon-btn shrink-0 border border-line" ' +
        'data-edit-env-del="' + i + '" aria-label="Remove variable ' + (i + 1) + '">' +
        svgIcon('x', 'h-4 w-4') + '</button>' +
        '</div>';
}

function renderEditEnv() {
    const w = $('#editEnvList');
    if (!_editEnvVars.length) {
        w.innerHTML = '<p class="rounded-lg border border-line bg-subtle px-3 py-2 text-xs text-zinc-500">No variables. Click Add to create one.</p>';
        return;
    }
    w.innerHTML = _editEnvVars.map(editEnvRowHtml).join('');
}

function appendEditEnvRow() {
    if (_editEnvLoading || !_editEnvAppId) return;
    const ctx = 'edit:' + _editEnvAppId;
    syncEditEnvFromDOM();
    _editEnvVars = envRecover(ctx, _editEnvVars);
    _editEnvVars.push({ k: '', v: '' });
    envDraftWrite(ctx, _editEnvVars);

    const w = $('#editEnvList');
    const i = _editEnvVars.length - 1;
    if (!w.querySelector('[data-edit-env-k]')) w.innerHTML = '';
    w.insertAdjacentHTML('beforeend', editEnvRowHtml(_editEnvVars[i], i));

    const ks = $$('[data-edit-env-k]', w);
    if (ks.length) ks[ks.length - 1].focus();
}

async function saveEditEnvAndRedeploy() {
    if (_editEnvBusy || !_editEnvAppId) return;

    syncEditEnvFromDOM();          // ← tambahan, supaya save selalu pakai nilai DOM
    const appId = _editEnvAppId;
    const reqId = _editEnvReqId;
    // Validasi key dulu — sebelum sentuh server


    let invalidKey = null;
    for (let i = 0; i < _editEnvVars.length; i++) {
        const k = (_editEnvVars[i].k || '').trim();
        if (k && !/^[A-Za-z_][A-Za-z0-9_]*$/.test(k)) { invalidKey = k; break; }
    }
    if (invalidKey) {
        toast('danger', 'Invalid env key', '"' + invalidKey + '" — hanya A-Z, a-z, 0-9, _. Tidak boleh mulai angka.');
        return;
    }

    _editEnvBusy = true;
    $('#btnEditEnvSave').disabled = true;
    $('#btnEditEnvCancel').disabled = true;
    $('#editEnvSpin').classList.remove('hidden');
    $('#editEnvLabel').textContent = 'Saving…';

    // Backend mengharapkan { "env": { "KEY": "VALUE" } }
    const envObj = {};
    _editEnvVars.forEach(function (e) {
        const k = (e.k || '').trim();
        if (k) envObj[k] = e.v;
    });

    const payload = { env: envObj };


    try {
        try {
            await apiFetch('/api/apps/' + appId + '/env', { method: 'PUT', body: payload });
            envDraftClear('edit:' + appId);
        } catch (e) {
            toast('danger', 'Save failed', e.message);
            return;
        }

        if (reqId === _editEnvReqId) $('#editEnvLabel').textContent = 'Redeploying…';

        let res;
        try {
            res = await apiFetch('/api/apps/' + appId + '/redeploy', { method: 'POST' });
        } catch (e) {
            toast('danger', 'Redeploy failed', 'Env for ' + appId + ' was saved, but redeploy failed: ' + e.message);
            return;
        }

        toast('success', 'Env saved', 'Redeploy queued for ' + appId + '.');
        if (reqId === _editEnvReqId) closeModal('modalEditEnv');

        const c = CONTAINERS.find(function (x) { return String(x.id) === String(appId); });
        const slug = c ? c.slug : 'app';
        const depId = res && res.deployment_id ? res.deployment_id : null;
        dpStart(appId, slug, depId, c ? c.sourceType : 'upload');
    } finally {
        _editEnvBusy = false;
        const modal = $('#modalEditEnv');
        if (modal && !modal.classList.contains('hidden')) {
            $('#btnEditEnvSave').disabled = _editEnvLoading;
            $('#btnEditEnvCancel').disabled = false;
            $('#editEnvSpin').classList.add('hidden');
            if (!_editEnvLoading) $('#editEnvLabel').textContent = 'Save & redeploy';
        }
    }
}
// Overview (real data from CONTAINERS + DEPLOYS)
function renderOverview() {
    $('#statServices').textContent = new Set(CONTAINERS.map(function (c) { return c.service; })).size;

    const running = CONTAINERS.filter(function (c) { return c.status === 'Running'; }).length;
    const restarting = CONTAINERS.filter(function (c) { return c.status === 'Restarting'; }).length;
    const failed = CONTAINERS.filter(function (c) { return c.status === 'Failed'; }).length;
    const stopped = CONTAINERS.filter(function (c) { return c.status === 'Stopped'; }).length;
    $('#statRunning').textContent = running;

    const parts = [];
    if (restarting) parts.push(restarting + ' restarting');
    if (failed) parts.push(failed + ' failed');
    if (stopped) parts.push(stopped + ' stopped');
    $('#statRunningSub').textContent = parts.length ? parts.join(' · ') : 'All containers healthy';

    const today = DEPLOYS.filter(function (d) { return dayKey(d.ts) === dayKey(Date.now()); }).length;
    $('#statDeploysToday').textContent = today;
    const lastSuc = DEPLOYS.find(function (d) { return d.status === 'Success'; });
    $('#statDeploysSub').textContent = 'Last success ' + (lastSuc ? rel(lastSuc.ts) : '—');

    // ── Deploy success rate (24h) ─────────────────────────────────────────
    // Diturunkan langsung dari DEPLOYS — tidak butuh endpoint baru.
    // Status yang dianggap sukses: "Success". Status final lainnya (Failed,
    // Canceled) masuk ke penyebut. Status in-flight (Building) DILEWATI
    // karena belum punya hasil final.
    const cutoff = Date.now() - 24 * 3600 * 1000;
    const last24h = DEPLOYS.filter(function (d) { return d.ts >= cutoff; });
    const finalized = last24h.filter(function (d) {
        return d.status === 'Success' || d.status === 'Failed' || d.status === 'Canceled';
    });
    const ok = finalized.filter(function (d) { return d.status === 'Success'; }).length;
    const total = finalized.length;
    const failCnt = total - ok;

    if (total === 0) {
        $('#statSuccessRate').textContent = '—';
        $('#statSuccessSub').textContent = 'No finalized deploys in the last 24h';
    } else {
        const pct = (ok / total) * 100;
        $('#statSuccessRate').textContent = pct.toFixed(1) + '%';
        $('#statSuccessSub').textContent =
            ok + ' success · ' + failCnt + ' failed of ' + total + ' deploys';
    }

    renderContainerRows($('#tblOverviewContainers'), CONTAINERS.filter(function (c) {
        return c.status === 'Running' || c.status === 'Restarting' || c.status === 'Building';
    }), false);

    $('#listActivity').innerHTML = DEPLOYS.slice(0, 5).map(function (d) {
        const m = DEPLOY_STATUS[d.status] || DEPLOY_STATUS.Canceled;
        return '<li class="flex items-center gap-3 border-b border-line py-3 last:border-b-0">' +
            '<span class="h-1.5 w-1.5 shrink-0 rounded-full ' + m[1] + (d.status === 'Building' ? ' animate-pulse' : '') + '"></span>' +
            '<div class="min-w-0 flex-1">' +
            '<p class="truncate text-sm text-zinc-200">' + esc(d.message) + '</p>' +
            '<p class="mt-0.5 truncate text-xs text-zinc-500">' + esc(d.repo) + ' · ' + esc(d.branch) + ' · <span class="font-mono">' + esc(d.sha) + '</span></p>' +
            '</div>' +
            '<div class="shrink-0 text-right">' +
            '<p class="text-xs text-zinc-400">' + rel(d.ts) + '</p>' +
            '<p class="mt-0.5 text-xs tabular-nums text-zinc-500">' + esc(d.duration) + '</p>' +
            '</div></li>';
    }).join('');
}

// Deployments page
function renderDeploymentsPage() {
    if (!DEPLOYS.length) {
        $('#tblDeploys').innerHTML = '<tr><td colspan="6" class="px-4 py-10 text-center text-xs text-zinc-500">No deployments yet.</td></tr>';
        return;
    }
    $('#tblDeploys').innerHTML = DEPLOYS.map(function (d) {
        return '<tr class="group h-12 border-b border-line transition-colors last:border-b-0 hover:bg-white/[0.02]">' +
            '<td class="px-4 py-2"><p class="text-sm text-zinc-100">' + esc(d.message) + '</p><p class="mt-0.5 text-xs text-zinc-500"><span class="font-mono">' + esc(d.sha) + '</span> · ' + esc(d.repo) + '</p></td>' +
            '<td class="whitespace-nowrap px-4 py-2"><span class="inline-flex items-center gap-1.5 text-xs text-zinc-300">' + svgIcon('git-branch', 'h-3.5 w-3.5 text-zinc-500') + esc(d.branch) + '</span></td>' +
            '<td class="px-4 py-2">' + providerChip(d.provider) + '</td>' +
            '<td class="px-4 py-2">' + deployBadge(d.status) + '</td>' +
            '<td class="whitespace-nowrap px-4 py-2 text-sm tabular-nums">' + esc(d.duration) + '</td>' +
            '<td class="whitespace-nowrap px-4 py-2 text-xs text-zinc-500">' + fmtDateTime(d.ts) + '</td>' +
            '</tr>';
    }).join('');
}

// Containers page
function renderContainersPage() {
    renderContainerRows($('#tblContainers'), CONTAINERS, true);
    const running = CONTAINERS.filter(function (c) { return c.status === 'Running'; }).length;
    $('#containersMeta').textContent = running + ' of ' + CONTAINERS.length + ' running';
}

// Integrations — provider cards
function confirmBox(p) {
    return '<div class="mt-4 rounded-lg border border-line bg-danger/10 px-3 py-2.5">' +
        '<div class="flex flex-wrap items-center gap-x-3 gap-y-2">' +
        '<p class="min-w-0 flex-1 text-xs text-red-300">Disconnect ' + providerName(p) + '? Repositories imported from ' +
        (p === 'github' ? 'github.com' : 'gitlab.com') + ' will stop receiving automatic deployments.</p>' +
        '<button type="button" data-cancel-disconnect class="btn-ghost px-2.5 py-1 text-xs">Cancel</button>' +
        '<button type="button" data-confirm-disconnect="' + p + '" class="rounded-lg bg-danger px-2.5 py-1 text-xs font-medium text-white transition-colors hover:bg-red-600">Disconnect</button>' +
        '</div></div>';
}

function providerCard(p) {
    const conn = connections[p];
    const name = providerName(p);
    const sub = p === 'github'
        ? 'Import repositories from github.com and deploy on every push.'
        : 'Connect gitlab.com to import projects and mirror pipelines.';

    let badge, body;

    if (conn.needsReconnect) {
        badge = '<span class="badge text-red-400"><span class="badge-dot bg-red-400"></span>Reconnect required</span>';
        body =
            '<div class="mt-4 rounded-lg border border-danger/40 bg-danger/10 px-3 py-2.5 text-xs text-red-300">' +
            'Otorisasi ' + name + ' sudah kedaluwarsa. Hubungkan ulang untuk melanjutkan sinkronisasi repo & deploy.' +
            '</div>' +
            '<div class="mt-4"><button type="button" data-connect="' + p + '" class="btn-primary">Reconnect ' + name + '</button></div>';
    } else if (conn.connected) {
        badge = '<span class="badge text-emerald-400"><span class="badge-dot bg-emerald-400"></span>Connected</span>';
        body =
            '<dl class="mt-4 grid grid-cols-[72px_1fr] items-baseline gap-y-1.5 text-xs">' +
            '<dt class="text-zinc-500">Account</dt><dd class="font-mono text-zinc-200">' + esc(conn.account) + '</dd>' +
            '<dt class="text-zinc-500">Scopes</dt><dd class="break-words font-mono text-zinc-200">' + esc(conn.scopes) + '</dd>' +
            '<dt class="text-zinc-500">Connected</dt><dd class="text-zinc-200">' + esc(conn.date) + '</dd>' +
            '</dl>' +
            '<div class="mt-4 flex items-center gap-2">' +
            '<button type="button" data-manage="' + p + '" class="btn-ghost">Manage</button>' +
            '<button type="button" data-disconnect="' + p + '" class="btn-danger">Disconnect</button>' +
            '</div>' + (ui.confirm === p ? confirmBox(p) : '');
    } else {
        badge = '<span class="badge text-zinc-400"><span class="badge-dot bg-zinc-500"></span>Not connected</span>';
        body = '<div class="mt-4"><button type="button" data-connect="' + p + '" class="btn-primary">Connect ' + name + '</button></div>';
    }

    return '<article class="card p-5">' +
        '<div class="flex items-start justify-between gap-3">' +
        '<div class="flex items-center gap-3">' +
        '<span class="text-zinc-100">' + providerLogo(p, 'h-6 w-6') + '</span>' +
        '<div><h3 class="text-sm font-medium text-zinc-100">' + name + '</h3>' +
        '<p class="mt-0.5 text-xs text-zinc-400">' + sub + '</p></div>' +
        '</div>' + badge +
        '</div>' + body + '</article>';
}

function renderProviderCards() {
    $('#providerCards').innerHTML = providerCard('github') + providerCard('gitlab');
}

// Integrations — repository list with live search + provider filter
function visibleRepos() {
    return REPOS.filter(function (r) { return r.provider !== 'gitlab' || connections.gitlab.connected; });
}
const EMPTY_SVG = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" class="h-12 w-12 text-zinc-600" aria-hidden="true"><circle cx="27" cy="26" r="12"/><path d="m36 35 10 10"/><path d="M16 52h24" stroke-dasharray="3 4"/><path d="M16 58h16" stroke-dasharray="3 4" opacity=".5"/></svg>';
function renderRepoList() {
    const q = $('#repoSearch').value.trim().toLowerCase();
    const pf = $('#repoFilter').value;
    const pool = visibleRepos().filter(function (r) { return pf === 'all' || r.provider === pf; });
    const rows = q ? pool.filter(function (r) { return r.name.toLowerCase().indexOf(q) > -1; }) : pool;
    $('#repoCount').textContent = rows.length + ' of ' + pool.length + ' repositories';
    const ul = $('#repoList');
    if (!rows.length) {
        const needle = q
            ? '<span class="font-mono text-zinc-200">&quot;' + esc(q) + '&quot;</span>'
            : '<span class="font-mono text-zinc-200">' + esc(pf === 'all' ? 'all providers' : providerName(pf)) + '</span>';
        const hint = pf === 'gitlab' && !connections.gitlab.connected
            ? '<p class="text-xs text-zinc-500">GitLab repositories appear after you connect GitLab.</p>'
            : '';
        ul.innerHTML = '<li class="px-4 py-12"><div class="mx-auto flex max-w-sm flex-col items-center gap-3 text-center">' +
            EMPTY_SVG +
            '<p class="text-sm text-zinc-400">No repositories match ' + needle + '</p>' + hint +
            '<button type="button" id="btnClearSearch" class="btn-outline">Clear search</button>' +
            '</div></li>';
        return;
    }
    ul.innerHTML = rows.map(function (r) {
        const parts = r.name.split('/');
        return '<li class="flex flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3 sm:px-5">' +
            '<div class="min-w-0 flex-1">' +
            '<p class="truncate text-sm"><span class="text-zinc-500">' + esc(parts[0]) + '/</span><span class="font-medium text-zinc-100">' + esc(parts[1]) + '</span></p>' +
            '<div class="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-zinc-500">' +
            providerChip(r.provider) +
            '<span class="inline-flex items-center gap-1.5"><span class="h-2 w-2 rounded-full" style="background:' + r.color + '"></span>' + r.lang + '</span>' +
            '<span>Updated ' + relHours(r.updatedH) + '</span>' +
            '</div>' +
            '</div>' +
            '<button type="button" data-import="' + r.name + '" class="btn-outline shrink-0">Import</button></li>';
    }).join('');
}

// Integrations — OAuth simulation
const SCOPES = {
    github: [
        { id: 'repo', d: 'Full control of private repositories', on: true },
        { id: 'admin:repo_hook', d: 'Create and manage repository webhooks', on: true },
        { id: 'read:org', d: 'Read organization membership', on: false }
    ],
    gitlab: [
        { id: 'api', d: 'Authenticated API access, including repositories', on: true },
        { id: 'read_repository', d: 'Read repository files and commits', on: true },
        { id: 'read_user', d: 'Read user profile and group membership', on: false }
    ]
};
let oauthP = 'github';
let oauthMode = 'connect';
function openOAuth(p, mode) {
    oauthP = p;
    oauthMode = mode;
    $('#oauthTitle').textContent = mode === 'connect' ? 'Authorize ' + providerName(p) : 'Manage ' + providerName(p) + ' access';
    $('#oauthDesc').textContent = mode === 'connect'
        ? 'W-deploy is requesting the following access to your ' + providerName(p) + ' account.'
        : 'Update the scopes granted to W-deploy on ' + providerName(p) + '.';
    $('#oauthLogo').innerHTML = providerLogo(p, 'h-5 w-5');
    const have = connections[p].scopes ? connections[p].scopes.split(', ') : [];
    $('#oauthScopes').innerHTML = SCOPES[p].map(function (s) {
        const on = mode === 'manage' ? have.indexOf(s.id) > -1 : s.on;
        return '<label class="flex cursor-pointer items-start gap-2.5 rounded-lg border border-line bg-subtle px-3 py-2.5 transition-colors hover:bg-line">' +
            '<input type="checkbox" class="mt-0.5 h-3.5 w-3.5 accent-accent" data-scope="' + s.id + '"' + (on ? ' checked' : '') + '>' +
            '<span class="min-w-0"><span class="block font-mono text-xs text-zinc-100">' + s.id + '</span>' +
            '<span class="block text-xs text-zinc-500">' + s.d + '</span></span></label>';
    }).join('');
    $('#oauthNote').textContent = 'You will be redirected to ' + (p === 'github' ? 'github.com' : 'gitlab.com') + ' to approve this request. No redirect happens in this demo.';
    $('#btnOAuthGo').textContent = mode === 'connect' ? 'Authorize' : 'Save scopes';
    $('#btnOAuthGo').disabled = !SCOPES[p].some(function (s) { return mode === 'manage' ? have.indexOf(s.id) > -1 : s.on; });
    openModal('modalOAuth');
}
function confirmOAuth() {
    const picked = $$('#oauthScopes input:checked').map(function (i) { return i.dataset.scope; });
    if (!picked.length) return;
    if (oauthMode === 'connect') {
        connections[oauthP] = {
            connected: true,
            account: '@acme-org',
            scopes: picked.join(', '),
            date: fmtDateTime(Date.now()),
            needsReconnect: false
        };
        toast('success', providerName(oauthP) + ' connected', 'Repositories are now being synced.');
    } else {
        connections[oauthP].scopes = picked.join(', ');
        toast('success', 'Scopes updated', providerName(oauthP) + ' access saved for W-deploy.');
    }
    closeModal('modalOAuth');
    renderRoute();
    const btn = $('[data-' + (connections[oauthP].connected ? 'manage' : 'connect') + '="' + oauthP + '"]');
    if (btn) btn.focus();
}

// Settings
const SECRET = 'whsec_9f2e7c41a8b3d605f1e9';
const SECRET_MASK = 'whsec_' + '·'.repeat(16);
let secretShown = false;

function settingsEnvRowHtml(e, i) {
    return '<div class="flex flex-col gap-2 sm:flex-row">' +
        '<input type="text" class="input font-mono sm:w-56" ' +
        'id="settingsEnvKey-' + i + '" name="settingsEnvKey-' + i + '" ' +
        'aria-label="Workspace variable name ' + (i + 1) + '" ' +
        'value="' + esc(e.k || '') + '" data-var-k="' + i + '" ' +
        'placeholder="KEY" list="envKeySuggestions" ' +
        'autocomplete="off" spellcheck="false">' +
        '<input type="text" class="input flex-1 font-mono" ' +
        'id="settingsEnvValue-' + i + '" name="settingsEnvValue-' + i + '" ' +
        'aria-label="Workspace variable value ' + (i + 1) + '" ' +
        'value="' + esc(e.v || '') + '" data-var-v="' + i + '" ' +
        'placeholder="value" autocomplete="off" spellcheck="false">' +
        '<button type="button" class="icon-btn shrink-0 self-start border border-line" ' +
        'data-var-del="' + i + '" aria-label="Remove variable ' + (i + 1) + '">' +
        svgIcon('x', 'h-4 w-4') + '</button>' +
        '</div>';
}

function renderEnvVars() {
    const w = $('#envList');
    if (!envVars.length) {
        w.innerHTML = '<p class="rounded-lg border border-line bg-subtle px-3 py-3 text-xs text-zinc-500">No environment variables. New containers will start with platform defaults only.</p>';
        return;
    }
    w.innerHTML = envVars.map(settingsEnvRowHtml).join('');
}

function settingsEnvCtx() { return 'settings:' + (currentAppId || 'workspace'); }
function syncSettingsEnvFromDOM() {
    const w = $('#envList');
    $$('[data-var-k]', w).forEach(function (inp) { const r = envVars[Number(inp.dataset.varK)]; if (r) r.k = inp.value; });
    $$('[data-var-v]', w).forEach(function (inp) { const r = envVars[Number(inp.dataset.varV)]; if (r) r.v = inp.value; });
}
function renderSettings() {
    let restored = false;
    if (!envHasContent(envVars)) {
        const saved = envDraftRead(settingsEnvCtx());
        if (saved && envHasContent(saved)) { envVars = saved; restored = true; }
    }
    renderEnvVars();
    secretShown = false;
    $('#inpSecret').value = SECRET_MASK;
    $('#btnReveal').setAttribute('aria-pressed', 'false');
    $('#btnReveal').setAttribute('aria-label', 'Reveal webhook secret');
    $('#icoEye').classList.remove('hidden');
    $('#icoEyeOff').classList.add('hidden');
    dirty = restored;
    $('#btnSave').disabled = !restored;
}
function markDirty() {
    dirty = true;
    $('#btnSave').disabled = false;
}

function toggleSecret() {
    secretShown = !secretShown;
    $('#inpSecret').value = secretShown ? SECRET : SECRET_MASK;
    $('#btnReveal').setAttribute('aria-pressed', String(secretShown));
    $('#btnReveal').setAttribute('aria-label', secretShown ? 'Hide webhook secret' : 'Reveal webhook secret');
    $('#icoEye').classList.toggle('hidden', secretShown);
    $('#icoEyeOff').classList.toggle('hidden', !secretShown);
}

// Toasts — bottom right, auto-dismiss 3s, max 3 stacked
function toast(type, title, msg) {
    const wrap = $('#toastWrap');
    while (wrap.children.length >= 3) wrap.firstElementChild.remove();
    const el = document.createElement('div');
    el.className = 'toast pointer-events-auto flex items-start gap-2.5 rounded-lg border border-line bg-panel px-3 py-2.5';
    const icon = type === 'success' ? 'circle-check' : (type === 'danger' ? 'alert-triangle' : 'info');
    const color = type === 'success' ? 'text-emerald-400' : (type === 'danger' ? 'text-red-400' : 'text-accent');
    el.innerHTML = '<span class="' + color + '">' + svgIcon(icon, 'h-4 w-4') + '</span>' +
        '<div class="min-w-0"><p class="text-xs font-medium text-zinc-100">' + esc(title) + '</p>' +
        (msg ? '<p class="mt-0.5 text-xs text-zinc-400">' + esc(msg) + '</p>' : '') + '</div>';
    wrap.appendChild(el);
    setTimeout(function () {
        el.classList.add('toast-out');
        setTimeout(function () { el.remove(); }, 190);
    }, 3000);
}

// Modal system — fade + scale transition, focus trap, Escape, body scroll lock
const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex="0"]';
const modalStack = [];
let locks = 0;
function lockBody(on) {
    locks = Math.max(0, locks + (on ? 1 : -1));
    document.body.classList.toggle('overflow-hidden', locks > 0);
}
function openModal(id) {
    const ov = document.getElementById(id);
    if (!ov || !ov.classList.contains('hidden')) return;
    ov.__opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    ov.classList.remove('hidden');
    ov.classList.add('flex');
    // Reset any persisted inner scroll position from a previous open
    ov.scrollTop = 0;
    $$('.overflow-y-auto, .overflow-auto', ov).forEach(function (el) { el.scrollTop = 0; });
    requestAnimationFrame(function () {
        requestAnimationFrame(function () { ov.classList.add('is-open'); });
    });
    lockBody(true);
    modalStack.push(id);
    const f = ov.querySelector(FOCUSABLE);
    if (f) f.focus();
}
function closeModal(id) {
    const ov = document.getElementById(id);
    if (!ov || ov.classList.contains('hidden')) return;
    if (id === 'modalEditEnv') {
        _editEnvReqId++;
        _editEnvLoading = false;
    }
    ov.classList.remove('is-open');
    const i = modalStack.indexOf(id);
    if (i > -1) modalStack.splice(i, 1);
    setTimeout(function () {
        ov.classList.add('hidden');
        ov.classList.remove('flex');
    }, 200);
    lockBody(false);
    if (ov.__opener) {
        ov.__opener.focus();
        ov.__opener = null;
    }
}
function trapTab(e) {
    if (e.key !== 'Tab') return;
    const f = $$(FOCUSABLE, this).filter(function (el) { return !el.closest('.hidden'); });
    if (!f.length) return;
    const first = f[0];
    const last = f[f.length - 1];
    if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
    else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
}

// Sidebar drawer (below lg)
let drawerOpen = false;
function openDrawer() {
    drawerOpen = true;
    $('#sidebar').classList.remove('-translate-x-full');
    $('#sidebar').classList.add('translate-x-0');
    $('#sidebarBackdrop').classList.remove('hidden');
    $('#btnSidebar').setAttribute('aria-expanded', 'true');
    lockBody(true);
}
function closeDrawer() {
    if (!drawerOpen) return;
    drawerOpen = false;
    $('#sidebar').classList.remove('translate-x-0');
    $('#sidebar').classList.add('-translate-x-full');
    $('#sidebarBackdrop').classList.add('hidden');
    $('#btnSidebar').setAttribute('aria-expanded', 'false');
    lockBody(false);
}

// Environment switcher
function setEnv(e2) {
    if (env === e2) return;
    env = e2;
    const prod = e2 === 'Production';
    [['#envProd', prod], ['#envStg', !prod]].forEach(function (pair) {
        const b = $(pair[0]);
        b.classList.toggle('bg-line', pair[1]);
        b.classList.toggle('text-zinc-100', pair[1]);
        b.classList.toggle('text-zinc-400', !pair[1]);
        b.setAttribute('aria-pressed', String(pair[1]));
    });
    const lbl = $('#deployEnvLabel');
    if (lbl) lbl.textContent = e2.toLowerCase();
    toast('info', 'Environment switched', 'Dashboard now targets ' + e2.toLowerCase() + '.');
}

// Deploy modal — tabs, repository prefill, custom database dropdown
let deployTab = 'import';
let deployProvider = 'github';
function setTab(t) {
    deployTab = t;
    const imp = t === 'import';
    [[$('#tabImport'), $('#panelImport'), imp], [$('#tabLocal'), $('#panelLocal'), !imp]].forEach(function (triple) {
        const tb = triple[0];
        const pn = triple[1];
        const on = triple[2];
        tb.setAttribute('aria-selected', String(on));
        tb.tabIndex = on ? 0 : -1;
        tb.classList.toggle('bg-line', on);
        tb.classList.toggle('text-zinc-100', on);
        tb.classList.toggle('text-zinc-400', !on);
        pn.classList.toggle('hidden', !on);
    });
}

// resetDeploy — repopulate deploy modal fields
function resetDeploy(opts) {
    const o = opts || {};
    const repo = o.repo || null;
    const r = repo ? REPOS.find(function (x) { return x.name === repo; }) : null;
    deployProvider = r ? r.provider : (o.provider || 'github');
    $('#tabImport').textContent = 'Import from ' + providerName(deployProvider);
    setTab(o.tab || 'import');
    const list = REPOS.filter(function (x) { return x.provider === deployProvider; });
    $('#selRepo').innerHTML =
        '<option value="" disabled' + (repo ? '' : ' selected') + '>Select a repository</option>' +
        list.map(function (x) {
            return '<option value="' + esc(x.name) + '"' + (x.name === repo ? ' selected' : '') + '>' + esc(x.name) + '</option>';
        }).join('');
    $('#inpBranch').value = r ? r.defaultBranch : 'main';
    if ($('#selProjectType')) $('#selProjectType').value = '';

    $('#btnDeploy').disabled = false;
    $('#btnDeployCancel').disabled = false;
    $('#btnDeployX').disabled = false;
    $('#deploySpin').classList.add('hidden');
    $('#deployLabel').textContent = 'Deploy';
    $('#deployEnvLabel').textContent = env.toLowerCase();
    deployEnvVars = [];
    envDraftClear('deploy');      // mulai bersih: jangan bawa env app lain
    renderDeployEnv();
    $('#selDatabase').value = '';
    $('#envDetectHint').classList.add('hidden');
}
function openDeploy(opts) {
    resetDeploy(opts);
    openModal('modalDeploy');
}

// ─── Resource monitoring — real-time WebSocket ───────────────────────────────
//
// Three dedicated WS endpoints:
//   /api/ws/resource/cpu  → {usage: number, cores: number}
//   /api/ws/resource/ram  → {total, used, free, available, percent, swap_total, swap_used, swap_percent}
//   /api/ws/resource/disk → {total, used, free, percent, partitions:[...]}
// Every message is a full JSON snapshot — no delta handling required.

// Helpers
function fmtMB(mb) {
    if (mb >= 1024) return (mb / 1024).toFixed(1) + ' GB';
    return mb + ' MB';
}
function barColor(pct) {
    if (pct >= 90) return 'bg-danger';
    if (pct >= 70) return 'bg-warn';
    return 'bg-accent';
}
function setBarColor(bar, pct) {
    bar.classList.remove('bg-accent', 'bg-warn', 'bg-danger');
    bar.classList.add(barColor(pct));
}

// Timestamps per metrik — dipakai oleh tickLabels()
let _lastCpuUpdate = 0;
let _lastRamUpdate = 0;
let _lastDiskUpdate = 0;

// tickLabels memperbarui teks "Updated Xs ago" di tiap kartu resource.
// Dipanggil oleh setInterval(tickLabels, 1000) di init().
function tickLabels() {
    const map = [
        ['#resCpuUpd', _lastCpuUpdate],
        ['#resRamUpd', _lastRamUpdate],
        ['#resDiskUpd', _lastDiskUpdate],
    ];
    for (let i = 0; i < map.length; i++) {
        const el = $(map[i][0]);
        if (el && map[i][1]) el.textContent = 'Updated ' + rel(map[i][1]);
    }
}

// ── CPU ───────────────────────────────────────────────────────────────────────
function setCpuMetrics(data) {
    const pct = Math.round(data.usage * 10) / 10;
    const val = $('#resCpuVal');
    const bar = $('#resCpuBar');
    const det = $('#resCpuDetail');
    if (!val) return;
    val.textContent = pct + '%';
    bar.style.width = Math.min(100, pct) + '%';
    setBarColor(bar, pct);
    det.textContent = data.cores ? data.cores + ' cores' : '';
    _lastCpuUpdate = Date.now();
    tickLabels(); // update label langsung, tanpa menunggu tick berikutnya
}

function setRamMetrics(data) {
    const pct = Math.round(data.percent * 10) / 10;
    const val = $('#resRamVal');
    const bar = $('#resRamBar');
    const det = $('#resRamDetail');
    if (!val) return;
    val.textContent = pct + '%';
    bar.style.width = Math.min(100, pct) + '%';
    setBarColor(bar, pct);
    det.textContent = fmtMB(data.used) + ' of ' + fmtMB(data.total);
    _lastRamUpdate = Date.now();
    tickLabels();
}

function setDiskMetrics(data) {
    const pct = Math.round(data.percent * 10) / 10;
    const val = $('#resDiskVal');
    const bar = $('#resDiskBar');
    const det = $('#resDiskDetail');
    if (!val) return;
    val.textContent = pct + '%';
    bar.style.width = Math.min(100, pct) + '%';
    setBarColor(bar, pct);
    det.textContent = fmtMB(data.used) + ' of ' + fmtMB(data.total);
    _lastDiskUpdate = Date.now();
    tickLabels();
}
// wsURL membangun URL WebSocket lengkap dengan token JWT di query string.
// Browser tidak bisa mengirim header Authorization pada handshake WS,
// jadi backend membaca token dari ?token= (lihat middleware.RequireAuth).
function wsURL(path) {
    const apiURL = new URL(API_BASE);
    const proto = apiURL.protocol === 'https:' ? 'wss:' : 'ws:';
    let url = proto + '//' + apiURL.host + path;
    if (sessionToken) {
        url += (path.indexOf('?') >= 0 ? '&' : '?') + 'token=' + encodeURIComponent(sessionToken);
    }
    return url;
}

// ── WebSocket manager ─────────────────────────────────────────────────────────
function makeResourceWS(path, onData, onFail) {
    let ws, retryDelay = 1000, retryTimer = null, stopped = false;
    let failCount = 0;

    function connect() {
        if (stopped) return;
        ws = new WebSocket(wsURL(path));

        ws.onopen = function () {
            console.log('[ws] OPEN  ' + path);     // ← tambahan
            retryDelay = 1000;
            failCount = 0;
        };

        ws.onerror = function (e) {
            console.error('[ws] ERROR ' + path, e); // ← tambahan
        };

        ws.onclose = function (ev) {
            if (stopped) return;
            console.log('[ws] CLOSE ' + path +
                ' code=' + ev.code +
                ' reason=' + ev.reason +
                ' fail=' + (failCount + 1)); // ← tambahan
            failCount++;
            if (failCount === 3 && onFail) onFail();
            retryTimer = setTimeout(connect, retryDelay);
            retryDelay = Math.min(retryDelay * 2, 30000);
        };

        ws.onmessage = function (ev) {
            try {
                const data = JSON.parse(ev.data);
                failCount = 0;
                onData(data);
            } catch (e) {
                console.warn('[ws] parse error ' + path, e, ev.data);
            }
        };
    }

    connect();
    return {
        close: function () {
            stopped = true;
            if (retryTimer) clearTimeout(retryTimer);
            if (ws) try { ws.close(); } catch (_) { }
        }
    };
}
// Hold references so we can close on page unload (optional)
let _resWsCpu = null;
let _resWsRam = null;
let _resWsDisk = null;

function showResourceError(sel) {
    const el = $(sel);
    if (el) el.textContent = 'Connection failed — login atau cek server';
}

function startResourceMonitoring() {

    _resWsCpu = makeResourceWS('/api/ws/resource/cpu', setCpuMetrics,
        function () { showResourceError('#resCpuDetail'); });
    _resWsRam = makeResourceWS('/api/ws/resource/ram', setRamMetrics,
        function () { showResourceError('#resRamDetail'); });
    _resWsDisk = makeResourceWS('/api/ws/resource/disk', setDiskMetrics,
        function () { showResourceError('#resDiskDetail'); });

    console.log('Resource monitoring started ' + (_resWsCpu ? 'CPU' : 'No CPU') + ' ' + (_resWsRam ? 'RAM' : 'No RAM') + ' ' + (_resWsDisk ? 'Disk' : 'No Disk'));
}

// Graceful teardown on tab close / unload
window.addEventListener('beforeunload', function () {
    if (_resWsCpu) _resWsCpu.close();
    if (_resWsRam) _resWsRam.close();
    if (_resWsDisk) _resWsDisk.close();
    stopContainerStatsMonitoring();
});

// Hash router — no reload, active nav accent, breadcrumb + title sync
const ROUTES = { overview: 'Overview', deployments: 'Deployments', containers: 'Containers', integrations: 'Integrations', settings: 'Settings' };
function renderRoute() {
    const fns = {
        overview: renderOverview,
        deployments: renderDeploymentsPage,
        containers: renderContainersPage,
        integrations: renderIntegrations,
        settings: renderSettings
    };
    fns[current]();
}
function renderIntegrations() {
    renderProviderCards();
    renderRepoList();
}
function navigate(target) {
    if (typeof target === 'string' && target.trim()) {
        const cleanTarget = target.trim();
        if (location.hash !== '#/' + cleanTarget) {
            location.hash = '#/' + cleanTarget;
            return;
        }
    }
    let h = location.hash.replace(/^#\/?/, '').trim();
    if (!ROUTES[h]) h = 'overview';
    current = h;

    Object.keys(ROUTES).forEach(function (k) {
        const el = $('#view-' + k);
        if (el) el.classList.toggle('hidden', k !== h);   // ← tambahkan if
    });

    $$('a[data-nav]').forEach(function (a) {
        const on = a.dataset.nav === h;
        a.classList.toggle('bg-subtle', on);
        a.classList.toggle('text-zinc-100', on);
        a.classList.toggle('text-zinc-400', !on);
        const ico = a.querySelector('[data-nav-ico]');
        if (ico) {                                         // ← tambahkan if
            ico.classList.toggle('text-accent', on);
            ico.classList.toggle('text-zinc-500', !on);
        }
        if (on) a.setAttribute('aria-current', 'page');
        else a.removeAttribute('aria-current');
    });

    const crumb = $('#crumbPage');
    if (crumb) crumb.textContent = ROUTES[h];

    document.title = ROUTES[h] + ' · W-deploy';
    closeDrawer();
    renderRoute();
    window.scrollTo(0, 0);
}

// Delegated clicks inside main (tables, cards, repo list)
function mainClick(e) {
    const imp = e.target.closest('[data-import]');
    if (imp) {
        const r = REPOS.find(function (x) { return x.name === imp.dataset.import; });
        if (r) openDeploy({ tab: 'import', provider: r.provider, repo: r.name });
        return;
    }
    const act = e.target.closest('[data-act]');
    if (act) { containerAction(act.dataset.act, act.dataset.id); return; }
    const conn = e.target.closest('[data-connect]');
    if (conn) {
        const p = conn.dataset.connect;
        const needsRealReconnect = connections[p] && connections[p].needsReconnect;

        // Reconnect GitHub = HARUS OAuth redirect asli. Mock modal tidak akan
        // memperbarui token di server → 401 GITHUB_REAUTH_REQUIRED akan muncul lagi.
        if (p === 'github' && needsRealReconnect) {
            toast('info', 'Redirecting to GitHub', 'Mengotorisasi ulang akses…');
            window.location.href = API_BASE + '/api/auth/github';
            return;
        }

        // Connect pertama kali / provider lain: mock modal boleh (belum diimplementasi
        // OAuth-nya). Tapi kalau nanti backend sudah support, ganti dengan redirect juga.
        openOAuth(p, 'connect');
        return;
    }
    const mg = e.target.closest('[data-manage]');
    if (mg) { openOAuth(mg.dataset.manage, 'manage'); return; }
    const dc = e.target.closest('[data-disconnect]');
    if (dc) {
        ui.confirm = dc.dataset.disconnect;
        renderProviderCards();
        const cf = $('[data-confirm-disconnect="' + ui.confirm + '"]');
        if (cf) cf.focus();
        return;
    }
    const cc = e.target.closest('[data-cancel-disconnect]');
    if (cc) {
        ui.confirm = null;
        renderProviderCards();
        return;
    }
    const cd = e.target.closest('[data-confirm-disconnect]');
    if (cd) {
        const p = cd.dataset.confirmDisconnect;
        connections[p] = { connected: false, account: '', scopes: '', date: '' };
        ui.confirm = null;
        renderProviderCards();
        toast('info', providerName(p) + ' disconnected', 'Connected repositories stopped syncing.');
        return;
    }
    if (e.target.closest('#btnClearSearch')) {
        $('#repoSearch').value = '';
        $('#repoFilter').value = 'all';
        renderRepoList();
        $('#repoSearch').focus();
    }
}

// Helper to read active DOM values into state before re-rendering
function syncEditEnvFromDOM() {
    const kInputs = $$('[data-edit-env-k]', $('#editEnvList'));
    const vInputs = $$('[data-edit-env-v]', $('#editEnvList'));
    kInputs.forEach(function (inp, i) {
        if (_editEnvVars[i]) _editEnvVars[i].k = inp.value;
    });
    vInputs.forEach(function (inp, i) {
        if (_editEnvVars[i]) _editEnvVars[i].v = inp.value;
    });
}

function syncDeployEnvFromDOM() {
    const kInputs = $$('[data-deploy-env-k]', $('#deployEnvList'));
    const vInputs = $$('[data-deploy-env-v]', $('#deployEnvList'));
    kInputs.forEach(function (inp, i) {
        if (deployEnvVars[i]) deployEnvVars[i].k = inp.value;
    });
    vInputs.forEach(function (inp, i) {
        if (deployEnvVars[i]) deployEnvVars[i].v = inp.value;
    });
}

// Init — bind everything once
function init() {
    $('#skipLink').addEventListener('click', function (e) {
        e.preventDefault();
        const m = $('#main');
        m.setAttribute('tabindex', '-1');
        m.focus({ preventScroll: true });
    });
    $('#btnSidebar').addEventListener('click', function () { drawerOpen ? closeDrawer() : openDrawer(); });
    $('#sidebarBackdrop').addEventListener('click', closeDrawer);
    $('#envProd').addEventListener('click', function () { setEnv('Production'); });
    $('#envStg').addEventListener('click', function () { setEnv('Staging'); });
    $('#btnNewDeployTop').addEventListener('click', function () { openDeploy(); });
    $('#btnNewDeployPage').addEventListener('click', function () { openDeploy(); });

    // Deploy modal
    $('#btnDeployX').addEventListener('click', function () { closeModal('modalDeploy'); });
    $('#btnDeployCancel').addEventListener('click', function () { closeModal('modalDeploy'); });
    $('#btnDeploy').addEventListener('click', deployNow);
    $('#tabImport').addEventListener('click', function () { setTab('import'); });
    $('#tabLocal').addEventListener('click', function () { setTab('local'); });
    $('#tablist').addEventListener('keydown', function (e) {
        if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
            e.preventDefault();
            setTab(deployTab === 'import' ? 'local' : 'import');
            $('.tab-btn[aria-selected="true"]').focus();
        }
    });
    $('#selRepo').addEventListener('change', function () {
        const r = REPOS.find(function (x) { return x.name === $('#selRepo').value; });
        if (r) $('#inpBranch').value = r.defaultBranch;
    });
    // Event listener untuk folder upload
    $('#inpLocalFiles').addEventListener('change', async function () {
        if (!this.files.length) {
            $('#uploadLabel').textContent = 'Click to select a folder';
            return;
        }

        // 1. Update label + kosongkan path manual
        const folderName = this.files[0].webkitRelativePath.split('/')[0] || 'folder';
        $('#uploadLabel').textContent = folderName + ' (' + this.files.length + ' files)';
        $('#inpLocalPath').value = '';

        // 2. Auto-detect .env
        const detected = await detectEnvFromFiles(this.files);
        if (!detected.length) {
            toast('info', 'No .env found', 'Tambahkan .env atau isi env manual.');
            return;
        }

        // 3. Merge — jangan timpa yang sudah ada manual
        syncDeployEnvFromDOM();
        const existing = new Set(
            deployEnvVars.filter(function (e) { return e.k; }).map(function (e) { return e.k; })
        );

        let added = 0;
        let skipped = 0;
        detected.forEach(function (e) {
            if (existing.has(e.k)) {
                skipped++;
                return;
            }
            deployEnvVars.push({ k: e.k, v: e.v });
            existing.add(e.k);
            added++;
        });
        envDraftWrite('deploy', deployEnvVars);

        renderDeployEnv();

        if (added) {
            $('#envDetectHint').classList.remove('hidden');
            const msg = added + ' variabel di-import' + (skipped ? ', ' + skipped + ' di-skip (sudah ada)' : '');
            toast('success', 'Env detected', msg);
        } else if (skipped) {
            toast('info', 'Env sudah ada', skipped + ' variabel sudah terisi sebelumnya.');
        }
    });

    // Deploy modal env vars
    $('#btnDeployAddEnv').addEventListener('click', appendDeployEnvRow);

    $('#deployEnvList').addEventListener('click', function (e) {
        const b = e.target.closest('[data-deploy-env-del]');
        if (!b) return;
        syncDeployEnvFromDOM(); // 1. Scrape typed values from DOM
        deployEnvVars = envRecover('deploy', deployEnvVars);
        deployEnvVars.splice(Number(b.dataset.deployEnvDel), 1);
        envDraftWrite('deploy', deployEnvVars);   // hapus juga dari storage
        renderDeployEnv();
    });

    $('#deployEnvList').addEventListener('input', function (e) {
        const t = e.target;
        if (t.dataset.deployEnvK != null) deployEnvVars[Number(t.dataset.deployEnvK)].k = t.value;
        else if (t.dataset.deployEnvV != null) deployEnvVars[Number(t.dataset.deployEnvV)].v = t.value;
        envDraftWrite('deploy', deployEnvVars);
    });
    // Saat fokus meninggalkan field (blur), simpan ke storage.
    $('#deployEnvList').addEventListener('focusout', function () {
        syncDeployEnvFromDOM();
        envDraftWrite('deploy', deployEnvVars);
    });

    // Edit Env modal
    $('#btnEditEnvX').addEventListener('click', function () { closeModal('modalEditEnv'); });
    $('#btnEditEnvCancel').addEventListener('click', function () { closeModal('modalEditEnv'); });
    $('#btnEditEnvSave').addEventListener('click', saveEditEnvAndRedeploy);

    $('#btnEditEnvAdd').addEventListener('click', appendEditEnvRow);

    $('#editEnvList').addEventListener('click', function (e) {
        const b = e.target.closest('[data-edit-env-del]');
        if (!b) return;
        syncEditEnvFromDOM();          // ← WAJIB, sebelumnya tidak ada
        const ctx = 'edit:' + _editEnvAppId;
        _editEnvVars = envRecover(ctx, _editEnvVars);
        _editEnvVars.splice(Number(b.dataset.editEnvDel), 1);
        envDraftWrite(ctx, _editEnvVars);         // hapus juga dari storage
        renderEditEnv();
    });

    $('#editEnvList').addEventListener('input', function (e) {
        const t = e.target;
        if (t.dataset.editEnvK != null) _editEnvVars[Number(t.dataset.editEnvK)].k = t.value;
        else if (t.dataset.editEnvV != null) _editEnvVars[Number(t.dataset.editEnvV)].v = t.value;
        if (_editEnvAppId) envDraftWrite('edit:' + _editEnvAppId, _editEnvVars);
    });
    $('#editEnvList').addEventListener('focusout', function () {
        if (!_editEnvAppId || _editEnvLoading) return;
        syncEditEnvFromDOM();
        envDraftWrite('edit:' + _editEnvAppId, _editEnvVars);
    });



    // OAuth modal
    $('#btnOAuthX').addEventListener('click', function () { closeModal('modalOAuth'); });
    $('#btnOAuthCancel').addEventListener('click', function () { closeModal('modalOAuth'); });
    $('#btnOAuthGo').addEventListener('click', confirmOAuth);
    $('#oauthScopes').addEventListener('change', function () {
        $('#btnOAuthGo').disabled = !$$('#oauthScopes input:checked').length;
    });

    // Logs modal — close WebSocket when modal is dismissed
    function closeLogs() { closeLogStream(); closeModal('modalLogs'); }
    // Tombol Logout
    $('#btnLogout').addEventListener('click', function () {
        if (confirm('Are you sure you want to log out?')) {
            logoutUser();
        }
    });
    $('#btnLogsX').addEventListener('click', closeLogs);
    $('#btnLogsClose').addEventListener('click', closeLogs);

    // Repository search + filter
    $('#repoSearch').addEventListener('input', renderRepoList);
    $('#repoFilter').addEventListener('change', renderRepoList);

    // Settings — env vars per-app (currentAppId must be set before saving)
    $('#settingsForm').addEventListener('input', markDirty);
    $('#settingsForm').addEventListener('submit', function (e) {
        e.preventDefault();
        if (!dirty) return;
        if (currentAppId) {
            saveEnvVars(currentAppId);
        } else {
            envDraftClear(settingsEnvCtx());
            dirty = false;
            $('#btnSave').disabled = true;
            toast('success', 'Settings saved', 'Workspace defaults updated.');
        }
    });
    $('#btnAddVar').addEventListener('click', function () {
        const ctx = settingsEnvCtx();
        syncSettingsEnvFromDOM();
        envVars = envRecover(ctx, envVars);
        envVars.push({ k: '', v: '' });
        envDraftWrite(ctx, envVars);

        const w = $('#envList');
        const i = envVars.length - 1;
        if (!w.querySelector('[data-var-k]')) w.innerHTML = '';
        w.insertAdjacentHTML('beforeend', settingsEnvRowHtml(envVars[i], i));

        const ks = $$('[data-var-k]', w);
        if (ks.length) ks[ks.length - 1].focus();
        markDirty();
    });
    $('#envList').addEventListener('click', function (e) {
        const b = e.target.closest('[data-var-del]');
        if (!b) return;
        const ctx = settingsEnvCtx();
        syncSettingsEnvFromDOM();
        envVars = envRecover(ctx, envVars);
        const key = envVars[Number(b.dataset.varDel)] && envVars[Number(b.dataset.varDel)].k;
        if (currentAppId && key) {
            deleteEnvVar(currentAppId, key);
        } else {
            envVars.splice(Number(b.dataset.varDel), 1);
            envDraftWrite(ctx, envVars);          // hapus juga dari storage
            renderEnvVars();
            markDirty();
        }
    });
    $('#envList').addEventListener('input', function (e) {
        const t = e.target;
        if (t.dataset.varK != null) envVars[Number(t.dataset.varK)].k = t.value;
        else if (t.dataset.varV != null) envVars[Number(t.dataset.varV)].v = t.value;
        envDraftWrite(settingsEnvCtx(), envVars);
    });
    $('#envList').addEventListener('focusout', function () {
        syncSettingsEnvFromDOM();
        envDraftWrite(settingsEnvCtx(), envVars);
    });
    $('#btnReveal').addEventListener('click', toggleSecret);
    $('#btnCopySecret').addEventListener('click', function () {
        copyText(SECRET);
        const b = $('#btnCopySecret');
        b.textContent = 'Copied';
        setTimeout(function () { b.textContent = 'Copy'; }, 1500);
    });

    // Focus trap per modal + global Escape
    $$('.modal-overlay').forEach(function (ov) { ov.addEventListener('keydown', trapTab); });
    document.addEventListener('keydown', function (e) {
        if (e.key !== 'Escape') return;
        if (modalStack.length) {
            const top = modalStack[modalStack.length - 1];
            if (top === 'modalLogs') closeLogStream();
            closeModal(top);
        } else if (drawerOpen) closeDrawer();
    });

    // Generic Copy → Copied buttons
    document.addEventListener('click', function (e) {
        const b = e.target.closest('[data-copy]');
        if (!b) return;
        copyText(b.dataset.copy);
        if (b.__ct) clearTimeout(b.__ct);
        b.textContent = 'Copied';
        b.__ct = setTimeout(function () { b.textContent = 'Copy'; }, 1500);
    });

    $('#main').addEventListener('click', mainClick);
    window.addEventListener('hashchange', navigate);

    // Monitoring intervals (5s without motion, 2s otherwise)
    setInterval(tickLabels, 1000);

    // ── Bootstrap ────────────────────────────────────────────────────────
    // hapus 2 baris sebelum bootstrap
    captureOAuthSession();

    checkAuth().then(function (status) {
        if (status === 'unauthorized') { showLoginPrompt(); return; }
        if (status === 'error') {
            toast('danger', 'Server unreachable', 'Tidak dapat memverifikasi sesi.');
            navigate();
            return;
        }

        startResourceMonitoring();
        startContainerStatsMonitoring();
        console.log('[ws] resource monitoring started');

        return fetchApps()
            .then(function () { return Promise.all([fetchAllDeploys(), fetchRepos()]); })
            .finally(navigate);
    }).catch(function (err) {
        console.warn('Bootstrap error:', err);
        navigate();
    });
}

function showLoginPrompt() {
    // 1. Bersihkan semua timer auth (idempotent)
    if (sessionTimeoutTimer) { clearTimeout(sessionTimeoutTimer); sessionTimeoutTimer = null; }
    if (refreshTimeoutTimer) { clearTimeout(refreshTimeoutTimer); refreshTimeoutTimer = null; }
    if (refreshCheckInterval) { clearInterval(refreshCheckInterval); refreshCheckInterval = null; }

    // 2. Tutup background connections
    closeAllResourceWs();
    try { closeLogStream(); } catch (_) { }
    try { if (typeof _dpCleanup === 'function') _dpCleanup(); } catch (_) { }

    // 3. Jangan recreate kalau sudah ada
    if (document.getElementById('loginOverlay')) return;

    // 4. Cegah body scroll-lock sisa modal
    document.body.classList.remove('overflow-hidden');

    // 5. Render overlay
    const overlay = document.createElement('div');
    overlay.id = 'loginOverlay';
    overlay.className = 'fixed inset-0 z-[90] flex items-center justify-center bg-base';
    overlay.innerHTML = `
    <div class="mx-auto max-w-md space-y-6 px-6 text-center">
      <div class="mx-auto grid h-14 w-14 place-items-center rounded-xl border border-line bg-panel text-accent">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"
             stroke="currentColor" stroke-width="1.5" stroke-linecap="round"
             stroke-linejoin="round" class="h-6 w-6" aria-hidden="true">
          <path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4"/>
          <polyline points="10 17 15 12 10 7"/>
          <line x1="15" y1="12" x2="3" y2="12"/>
        </svg>
      </div>
      <div>
        <h1 class="text-lg font-medium text-zinc-100">Sign in to continue</h1>
        <p class="mt-1 text-sm text-zinc-400">Authenticate with GitHub to access the W-deploy console.</p>
      </div>
      <a href="${API_BASE}/api/auth/github" id="btnOverlaySignIn"
         class="btn-primary inline-flex">
        <svg viewBox="0 0 24 24" fill="currentColor" class="h-4 w-4" aria-hidden="true">
          <path d="${GH_PATH}"/>
        </svg>
        Sign in with GitHub
      </a>
      <div class="pt-2">
        <p class="text-xs text-zinc-500">Sign-in not working? Clear the local session and try again.</p>
        <button type="button" id="btnOverlaySignOut"
                class="btn-ghost mt-2 text-xs text-red-400 hover:bg-red-500/10 hover:text-red-300">
          Clear session &amp; sign out
        </button>
      </div>
    </div>`;

    // 6. Escape hatch — pakai logoutUser yang sudah ada
    overlay.querySelector('#btnOverlaySignOut').addEventListener('click', function (e) {
        e.preventDefault();
        const btn = e.currentTarget;
        btn.disabled = true;
        btn.textContent = 'Signing out…';
        logoutUser();   // ← ini reload setelah selesai, overlay hilang otomatis
    });

    // 7. Sign-in: redirect native + toast feedback
    overlay.querySelector('#btnOverlaySignIn').addEventListener('click', function () {
        toast('info', 'Redirecting to GitHub', 'Menyiapkan OAuth flow…');
    });

    document.body.appendChild(overlay);
}


init();