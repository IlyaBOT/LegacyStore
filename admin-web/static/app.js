(function () {
  "use strict";

  var state = {
    categories: [],
    apps: [],
    route: "home",
    os: "10.9.5",
    arch: "x86_64",
    category: "",
    downloads: [],
    currentUser: null,
    currentSession: null,
    pendingTwoFactorLogin: null,
    showAdmin: false
  };

  var main = document.getElementById("main");
  var sidebar = document.getElementById("sidebar");
  var searchForm = document.getElementById("searchForm");
  var searchInput = document.getElementById("searchInput");

  var osVersions = [
    "All Versions",
    "10.15",
    "10.14",
    "10.13",
    "10.12",
    "10.11",
    "10.10",
    "10.9.5",
    "10.8",
    "10.7",
    "10.6.8",
    "10.5.8"
  ];

  var accountRoutes = [
    ["login", "Sign In"],
    ["register", "Register"],
    ["profile", "Profile"],
    ["security", "Security"],
    ["legacy-passwords", "Legacy Passwords"]
  ];

  var adminRoutes = [
    ["uploads", "Uploads"],
    ["moderation", "Moderation Queue"],
    ["admin-dashboard", "Admin Dashboard"],
    ["users", "Users"],
    ["admin-apps", "Apps"],
    ["admin-versions", "Versions"],
    ["admin-artifacts", "Artifacts"],
    ["admin-reviews", "Reviews"],
    ["audit-log", "Audit Log"],
    ["settings", "Settings"]
  ];

  function api(path, options) {
    var requestOptions = Object.assign({ credentials: "same-origin" }, options || {});
    return fetch("/api/v1" + path, requestOptions).then(function (response) {
      return response.text().then(function (text) {
        var payload = text ? JSON.parse(text) : {};
        if (!response.ok) {
          var error = new Error(payload.error || response.statusText);
          error.status = response.status;
          error.payload = payload;
          throw error;
        }
        return payload;
      });
    });
  }

  function escapeHTML(value) {
    return String(value == null ? "" : value)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function routeTo(route) {
    window.location.hash = route;
  }

  function currentHash() {
    return window.location.hash.replace(/^#/, "") || "home";
  }

  function parseRoute() {
    var hash = currentHash();
    var parts = hash.split("?");
    var route = parts[0] || "home";
    var params = new URLSearchParams(parts[1] || "");
    return { route: route, params: params };
  }

  function appInitials(app) {
    return (app.name || app.slug || "LS")
      .split(/\s+/)
      .map(function (part) { return part.charAt(0); })
      .join("")
      .slice(0, 2)
      .toUpperCase();
  }

  function ratingDots(value) {
    var rating = Math.round(Number(value || 0));
    var html = "";
    for (var i = 1; i <= 5; i += 1) {
      html += '<span class="rating-dot' + (i > rating ? " empty" : "") + '"></span>';
    }
    return html;
  }

  function badges(items) {
    return (items || []).map(function (item, index) {
      var klass = index === 0 ? "badge gray" : "badge";
      return '<span class="' + klass + '">' + escapeHTML(item) + "</span>";
    }).join("");
  }

  function isBlockedApp(app) {
    return !!(app && app.compatibility && app.compatibility.status === "blocked");
  }

  function compatibilityPill(compatibility) {
    if (!compatibility || compatibility.status !== "blocked") {
      return "";
    }
    return '<span class="compat-pill">' + escapeHTML(compatibility.label || "Не совместимо") + "</span>";
  }

  function hasAdminAccess(user) {
    var roles = user && Array.isArray(user.roles) ? user.roles : [];
    var role = user && user.role ? user.role : "";
    return roles.indexOf("admin") >= 0 || roles.indexOf("moder") >= 0 || role === "admin" || role === "moder";
  }

  function setStatus(left, center, right) {
    document.getElementById("statusLeft").textContent = left == null ? "Showing: All Categories" : left;
    document.getElementById("statusCenter").textContent = center == null ? "" : center;
    document.getElementById("statusRight").textContent = right == null ? "Sort By: Featured" : right;
  }

  function setActiveTabs() {
    var topRoute = state.route.split("/")[0];
    document.querySelectorAll(".toolbar-tab").forEach(function (button) {
      var route = button.getAttribute("data-route");
      button.classList.toggle("is-active", route === topRoute || (topRoute === "catalog" && route === "categories"));
    });
  }

  function renderSidebar() {
    var categoryItems = state.categories.map(function (category) {
      return sourceButton(category.name, "category/" + category.slug, "category", state.category === category.slug);
    }).join("");

    var osItems = osVersions.map(function (version) {
      var active = version === "All Versions" ? state.os === "" : state.os === version;
      return '<button class="source-item' + (active ? " is-active" : "") + '" data-os="' + escapeHTML(version) + '" type="button">' +
        '<span class="source-icon source-icon-macos"></span><span>' + escapeHTML(version === "All Versions" ? "All Versions" : "macOS " + version) + "</span></button>";
    }).join("");

    var adminSection = state.showAdmin ?
      '<section class="source-section">' +
      '<div class="source-heading">Administration</div>' +
      adminRoutes.map(function (item) {
        return sourceButton(item[1], item[0], "admin", state.route === item[0]);
      }).join("") +
      '</section>' : "";

    sidebar.innerHTML =
      '<section class="source-section">' +
      '<div class="source-heading">Account</div>' +
      accountRoutes.map(function (item) {
        return sourceButton(item[1], item[0], "account", state.route === item[0]);
      }).join("") +
      '</section>' +
      adminSection +
      '<section class="source-section">' +
      '<div class="source-heading">Categories</div>' +
      sourceButton("All Categories", "categories", "category", state.route === "categories") +
      sourceButton("All Software", "catalog", "category", state.route === "catalog" && !state.category) +
      categoryItems +
      '</section>' +
      '<section class="source-section">' +
      '<div class="source-heading">macOS Versions</div>' +
      osItems +
      '</section>';

    sidebar.querySelectorAll("[data-route]").forEach(function (button) {
      button.addEventListener("click", function () {
        routeTo(button.getAttribute("data-route"));
      });
    });

    sidebar.querySelectorAll("[data-os]").forEach(function (button) {
      button.addEventListener("click", function () {
        var value = button.getAttribute("data-os");
        state.os = value === "All Versions" ? "" : value;
        render();
      });
    });
  }

  function sourceButton(label, route, icon, active, badge) {
    var iconClass = "source-icon source-icon-" + escapeHTML(icon || "category");
    return '<button class="source-item' + (active ? " is-active" : "") + '" data-route="' + escapeHTML(route) + '" type="button">' +
      '<span class="' + iconClass + '"></span>' +
      '<span>' + escapeHTML(label) + "</span>" +
      (badge != null ? '<span class="source-badge">' + escapeHTML(badge) + "</span>" : "") +
      "</button>";
  }

  function appCard(app) {
    var blocked = isBlockedApp(app);
    return '<article class="app-card' + (blocked ? " is-incompatible" : "") + '">' +
      '<div class="app-icon' + (blocked ? " is-incompatible" : "") + '">' + escapeHTML(appInitials(app)) + "</div>" +
      '<h3>' + escapeHTML(app.name) + "</h3>" +
      '<div class="app-meta">' + escapeHTML(app.category || "") + "</div>" +
      '<div class="rating-row">' + ratingDots(app.rating) + '<span class="muted">(' + escapeHTML(app.rating_count || 0) + ")</span></div>" +
      '<div class="badge-row">' + badges(app.arch_badges) + compatibilityPill(app.compatibility) + '</div>' +
      '<button class="metal-button" data-route="app/' + escapeHTML(app.slug) + '" type="button">Загрузить</button>' +
      "</article>";
  }

  function appRow(app) {
    var blocked = isBlockedApp(app);
    return '<article class="app-row">' +
      '<div class="app-icon small' + (blocked ? " is-incompatible" : "") + '">' + escapeHTML(appInitials(app)) + "</div>" +
      '<div><div class="list-name">' + escapeHTML(app.name) + "</div><div class=\"muted\">" + escapeHTML(app.summary || "") + "</div></div>" +
      '<div><strong>' + escapeHTML(app.category || "") + '</strong><div class="muted">Version ' + escapeHTML(app.recommended_version || "") + "</div></div>" +
      '<div><div class="badge-row">' + badges(app.arch_badges) + compatibilityPill(app.compatibility) + "</div></div>" +
      '<div><button class="metal-button" data-route="app/' + escapeHTML(app.slug) + '" type="button">Загрузить</button></div>' +
      "</article>";
  }

  function bindRouteButtons(root) {
    root.querySelectorAll("[data-route]").forEach(function (button) {
      button.addEventListener("click", function () {
        routeTo(button.getAttribute("data-route"));
      });
    });
  }

  function queryTarget() {
    var params = [
      "os=" + encodeURIComponent(state.os || "10.9.5"),
      "arch=" + encodeURIComponent(state.arch)
    ];
    if (state.os) {
      params.push("compatible=1");
    }
    return params.join("&");
  }

  function loadApps(category, limit) {
    var path = "/apps?" + queryTarget() + "&limit=" + encodeURIComponent(limit || 24) + "&page=1";
    if (category) {
      path += "&category=" + encodeURIComponent(category);
    }
    return api(path).then(function (payload) {
      return payload.apps || [];
    });
  }

  function renderHome() {
    state.category = "";
    main.innerHTML = '<div class="loading">Loading...</div>';
    Promise.all([loadApps("", 12), loadApps("graphics-design", 6)]).then(function (sets) {
      state.apps = sets[0];
      var featured = state.apps.find(function (app) { return app.slug === "pixelmator"; }) || state.apps[0] || {};
      main.innerHTML =
        '<section class="hero">' +
        '<div class="hero-content">' +
        '<h1>' + escapeHTML(featured.name || "LegacyStore") + "</h1>" +
        '<p>' + escapeHTML(featured.summary || "Classic software catalog for old Intel Macs.") + "</p>" +
        '<button class="blue-button" data-route="app/' + escapeHTML(featured.slug || "pixelmator") + '" type="button">Загрузить</button>' +
        '</div></section>' +
        panel("New and Noteworthy", "catalog", sets[0].slice(0, 6).map(appCard).join("")) +
        panel("Recommended for Your Mac", "catalog", sets[1].slice(0, 6).map(appCard).join(""));
      bindRouteButtons(main);
      setStatus("Showing: Featured", "1-12 of " + state.apps.length + " items", "Sort By: Featured");
      renderSidebar();
    }).catch(renderError);
  }

  function panel(title, route, body) {
    return '<section class="section-panel">' +
      '<div class="section-header"><h2>' + escapeHTML(title) + '</h2><button class="link-button" data-route="' + escapeHTML(route) + '" type="button">See All</button></div>' +
      '<div class="app-strip">' + (body || '<div class="empty-state">No items</div>') + "</div></section>";
  }

  function renderCatalog(category) {
    state.category = category || "";
    main.innerHTML = '<div class="loading">Loading...</div>';
    loadApps(state.category, 50).then(function (apps) {
      state.apps = apps;
      var title = categoryName(state.category) || "All Software";
      main.innerHTML =
        '<div class="view-title"><h1>' + escapeHTML(title) + '</h1>' + filtersHTML() + '</div>' +
        '<div class="app-list">' + (apps.length ? apps.map(appRow).join("") : '<div class="empty-state">No items</div>') + "</div>";
      bindRouteButtons(main);
      bindFilters();
      setStatus("Showing: " + title, apps.length + " items", "Sort By: Name");
      renderSidebar();
    }).catch(renderError);
  }

  function filtersHTML() {
    return '<div class="filter-bar">' +
      '<select id="osFilter" aria-label="macOS version">' +
      osVersions.map(function (version) {
        var value = version === "All Versions" ? "" : version;
        return '<option value="' + escapeHTML(value) + '"' + (state.os === value ? " selected" : "") + ">" + escapeHTML(version) + "</option>";
      }).join("") +
      '</select>' +
      '<select id="archFilter" aria-label="Architecture">' +
      '<option value="x86_64"' + (state.arch === "x86_64" ? " selected" : "") + ">x86_64</option>" +
      '<option value="i386"' + (state.arch === "i386" ? " selected" : "") + ">i386</option>" +
      '</select></div>';
  }

  function bindFilters() {
    var osFilter = document.getElementById("osFilter");
    var archFilter = document.getElementById("archFilter");
    if (osFilter) {
      osFilter.addEventListener("change", function () {
        state.os = osFilter.value;
        render();
      });
    }
    if (archFilter) {
      archFilter.addEventListener("change", function () {
        state.arch = archFilter.value;
        render();
      });
    }
  }

  function categoryName(slug) {
    var found = state.categories.find(function (category) { return category.slug === slug; });
    return found ? found.name : "";
  }

  function renderCategories() {
    state.category = "";
    var body = state.categories.map(function (category) {
      return '<button class="category-tile" data-route="category/' + escapeHTML(category.slug) + '" type="button">' +
        '<span class="source-icon"></span><strong>' + escapeHTML(category.name) + '</strong></button>';
    }).join("");
    main.innerHTML = '<div class="view-title"><h1>Categories</h1>' + filtersHTML() + '</div><div class="category-grid">' + body + "</div>";
    bindRouteButtons(main);
    bindFilters();
    setStatus("Showing: All Categories", state.categories.length + " categories", "Sort By: Name");
    renderSidebar();
  }

  function renderSearch(query) {
    state.category = "";
    main.innerHTML = '<div class="loading">Loading...</div>';
    api("/search?q=" + encodeURIComponent(query) + "&" + queryTarget()).then(function (payload) {
      var results = payload.results || [];
      main.innerHTML =
        '<div class="view-title"><h1>Search</h1>' + filtersHTML() + '</div>' +
        '<div class="app-list">' + (results.length ? results.map(appRow).join("") : '<div class="empty-state">No items</div>') + "</div>";
      bindRouteButtons(main);
      bindFilters();
      setStatus('Search: "' + query + '"', results.length + " items", "Sort By: Relevance");
      renderSidebar();
    }).catch(renderError);
  }

  function renderApp(slug) {
    state.category = "";
    main.innerHTML = '<div class="loading">Loading...</div>';
    api("/apps/" + encodeURIComponent(slug) + "?" + queryTarget()).then(function (app) {
      var artifact = app.recommended_artifact || {};
      var reasons = app.compatibility && app.compatibility.reasons ? app.compatibility.reasons : [];
      var isBlocked = app.compatibility && app.compatibility.status === "blocked";
      main.innerHTML =
        '<section class="detail-head">' +
        '<div class="app-icon' + (isBlocked ? " is-incompatible" : "") + '">' + escapeHTML(appInitials(app)) + "</div>" +
        '<div><h1>' + escapeHTML(app.name) + '</h1><div class="muted">' + escapeHTML(app.developer_name || "") + '</div>' +
        '<p>' + escapeHTML(app.summary || "") + '</p><div class="badge-row">' + badges(artifact.archs || []) + '</div></div>' +
        '<div class="detail-actions">' +
        '<button class="blue-button" id="downloadButton" type="button">Загрузить</button>' +
        '<button class="metal-button" data-route="catalog" type="button">Back to Catalog</button>' +
        '</div></section>' +
        '<div class="detail-grid">' +
        '<section class="detail-copy"><h2>Description</h2><p>' + escapeHTML(app.description || "") + '</p>' + compatibilityBlock(app.compatibility) + versionsTable(app.versions || []) + reviewsHTML(app.slug) + '</section>' +
        '<aside class="side-box"><h2>Information</h2>' +
        infoRow("Category", app.category) +
        infoRow("Version", artifact.version) +
        infoRow("Compatibility", isBlocked ? app.compatibility.label : "") +
        infoRow("Size", formatSize(artifact.size_bytes)) +
        infoRow("Package", artifact.package_type) +
        infoRow("SHA-256", artifact.sha256) +
        (reasons.length ? '<h2>Compatibility</h2><ul class="setting-list">' + reasons.map(function (r) { return '<li><span>' + escapeHTML(r.message) + '</span></li>'; }).join("") + "</ul>" : "") +
        '</aside></div>';
      bindRouteButtons(main);
      var downloadButton = document.getElementById("downloadButton");
      if (downloadButton) {
        downloadButton.disabled = !artifact.id || isBlocked;
        downloadButton.addEventListener("click", function () {
          loadDownloadMetadata(artifact.id, app);
        });
      }
      loadReviews(app.slug);
      setStatus("Showing: " + app.name, "1 item", "Version: " + (artifact.version || ""));
      renderSidebar();
    }).catch(renderError);
  }

  function reviewsHTML(slug) {
    var form = state.currentUser ? '<form id="reviewForm" class="review-form" data-slug="' + escapeHTML(slug) + '">' +
      '<h2>Your Review</h2><div class="filter-bar"><select name="rating" aria-label="Rating"><option value="5">5</option><option value="4">4</option><option value="3">3</option><option value="2">2</option><option value="1">1</option></select>' +
      '<input name="title" type="text" aria-label="Title"></div><div class="form-row"><label>Review</label><textarea name="body"></textarea></div><button class="blue-button" type="submit">Post Review</button></form>' : "";
    return '<section class="reviews-section">' + form + '<h2>Reviews</h2><div id="reviewsList" class="loading">Loading...</div></section>';
  }

  function loadReviews(slug) {
    var target = document.getElementById("reviewsList");
    if (!target) {
      return;
    }
    var form = document.getElementById("reviewForm");
    if (form) {
      form.addEventListener("submit", function (event) {
        event.preventDefault();
        var data = formJSON(form);
        data.rating = Number(data.rating || 5);
        api("/apps/" + encodeURIComponent(slug) + "/reviews", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(data) }).then(function () {
          loadReviews(slug);
        });
      });
    }
    api("/apps/" + encodeURIComponent(slug) + "/reviews").then(function (payload) {
      var reviews = payload.reviews || [];
      target.innerHTML = reviews.length ? '<ul class="setting-list">' + reviews.map(function (review) {
        return '<li><span><strong>' + escapeHTML(review.author || "User") + '</strong> ' + ratingDots(review.rating) + '<br>' + escapeHTML(review.title || "") + '<br><small>' + escapeHTML(review.body || "") + '</small></span><span>' + escapeHTML(review.likes || 0) + ' likes</span></li>';
      }).join("") + "</ul>" : '<div class="empty-state compact">No reviews</div>';
    }).catch(function () {
      target.innerHTML = '<div class="empty-state compact">Unable to load reviews</div>';
    });
  }

  function compatibilityBlock(compatibility) {
    if (!compatibility) {
      return "";
    }
    if (compatibility.status !== "blocked") {
      return "";
    }
    return '<p class="notice warning">Этот продукт не совместим с вашим Macintosh.</p>';
  }

  function versionsTable(versions) {
    if (!versions.length) {
      return "";
    }
    var rows = versions.map(function (version) {
      var artifact = (version.artifacts || [])[0] || {};
      if (state.os && artifact.compatibility_status === "blocked") {
        return "";
      }
      var status = artifact.compatibility_status === "blocked" ? (artifact.compatibility_label || "Не совместимо") : "";
      return "<tr><td>" + escapeHTML(version.version) + "</td><td>" + escapeHTML(version.release_date || "") + "</td><td>" +
        escapeHTML((artifact.archs || []).join(", ")) + "</td><td>" + escapeHTML(status) + "</td></tr>";
    }).join("");
    if (!rows) {
      return '<h2>Versions</h2><div class="empty-state compact">No compatible versions</div>';
    }
    return '<h2>Versions</h2><table class="version-table"><thead><tr><th>Version</th><th>Date</th><th>Arch</th><th>Status</th></tr></thead><tbody>' + rows + "</tbody></table>";
  }

  function infoRow(label, value) {
    if (!value) {
      return "";
    }
    return '<p><strong>' + escapeHTML(label) + ':</strong> <span class="muted">' + escapeHTML(value) + "</span></p>";
  }

  function formatSize(bytes) {
    bytes = Number(bytes || 0);
    if (!bytes) {
      return "";
    }
    if (bytes > 1024 * 1024 * 1024) {
      return (bytes / (1024 * 1024 * 1024)).toFixed(1) + " GB";
    }
    return (bytes / (1024 * 1024)).toFixed(1) + " MB";
  }

  function loadDownloadMetadata(id, app) {
    api("/download/" + encodeURIComponent(id) + "?" + queryTarget()).then(function (metadata) {
      state.downloads.unshift({
        name: app.name,
        version: metadata.version,
        file: metadata.file_name,
        size: formatSize(metadata.size_bytes),
        sha256: metadata.sha256,
        source: metadata.download_url || metadata.external_page_url || metadata.source_type
      });
      renderDownloads();
    }).catch(renderError);
  }

  function renderDownloads() {
    state.route = "downloads";
    var rows = state.downloads.map(function (item) {
      return '<div class="download-row"><div><strong>' + escapeHTML(item.name) + " " + escapeHTML(item.version || "") + '</strong><div class="muted">' +
        escapeHTML(item.file || "") + '</div></div><div>' + escapeHTML(item.size || "") + '</div><div><button class="metal-button" type="button">Metadata</button></div></div>';
    }).join("");
    main.innerHTML = '<div class="view-title"><h1>Downloads</h1></div><div class="download-list">' + (rows || '<div class="empty-state">No items</div>') + "</div>";
    setStatus("Showing: Downloads", state.downloads.length + " items", "Sort By: Recent");
    renderSidebar();
  }

  function renderUpdates() {
    main.innerHTML = '<div class="view-title"><h1>Updates</h1></div><section class="section-panel"><div class="section-header"><h2>Available Updates</h2></div><div class="empty-state">No items</div></section>';
    setStatus("Showing: Updates", "0 items", "Sort By: Date");
    renderSidebar();
  }

  function renderTopCharts() {
    state.category = "";
    loadApps("", 50).then(function (apps) {
      apps.sort(function (a, b) { return Number(b.rating_count || 0) - Number(a.rating_count || 0); });
      main.innerHTML = '<div class="view-title"><h1>Top Charts</h1>' + filtersHTML() + '</div><div class="app-list">' + apps.map(appRow).join("") + "</div>";
      bindRouteButtons(main);
      bindFilters();
      setStatus("Showing: Top Charts", apps.length + " items", "Sort By: Rating");
      renderSidebar();
    }).catch(renderError);
  }

  function renderAccount(route) {
    var titles = {
      login: "Sign In",
      register: "Register",
      profile: "Profile",
      security: "Security",
      "legacy-passwords": "Legacy Passwords"
    };
    var form = route === "register" ? registerForm() : loginForm();
    if (route === "profile") {
      form = profilePanel();
    }
    if (route === "security") {
      form = securityPanel();
    }
    if (route === "legacy-passwords") {
      form = legacyPasswordsPanel();
    }
    if (route === "login" || route === "register") {
      main.classList.add("is-auth");
      main.innerHTML = form;
    } else {
      main.innerHTML = '<div class="view-title"><h1>' + escapeHTML(titles[route]) + '</h1></div>' + form;
    }
    bindAuthForms();
    if (route === "security") {
      loadSecurityData();
    }
    if (route === "legacy-passwords") {
      loadLegacyData();
    }
    setStatus("Showing: Account", titles[route], "");
    renderSidebar();
  }

  function loginForm() {
    return '<div class="auth-window"><section class="account-panel auth-panel"><h2>Sign In</h2>' +
      '<form id="loginForm"><div class="form-row"><label>Email</label><input name="email" type="email" required></div>' +
      '<div class="form-row"><label>Password</label><input name="password" type="password" required></div>' +
      '<label class="check-row"><input name="remember_me" type="checkbox" value="1"><span>Remember Me</span></label>' +
      '<div id="authNotice" class="form-message" aria-live="polite"></div>' +
      '<button class="blue-button" type="submit">Sign In</button></form></section></div>';
  }

  function twoFactorLoginForm(email) {
    return '<div class="auth-window auth-window-small"><section class="account-panel auth-panel"><h2>Two-Factor Authentication</h2>' +
      '<form id="twoFactorLoginForm"><p class="auth-summary">' + escapeHTML(email) + '</p>' +
      '<div class="form-row"><label>Code</label><input name="totp_code" inputmode="numeric" autocomplete="one-time-code" required autofocus></div>' +
      '<div id="twoFactorLoginNotice" class="form-message" aria-live="polite"></div>' +
      '<div class="filter-bar"><button class="blue-button" type="submit">Verify</button><button class="metal-button" id="cancel2FALoginButton" type="button">Back</button></div></form></section></div>';
  }

  function registerForm() {
    return '<div class="auth-window"><section class="account-panel auth-panel"><h2>Register</h2>' +
      '<form id="registerForm"><div class="form-row"><label>Email</label><input name="email" type="email" required></div>' +
      '<div class="form-row"><label>Nickname</label><input name="nickname" type="text" required></div>' +
      '<div class="form-row"><label>Password</label><input name="password" type="password" required></div>' +
      '<label class="check-row"><input name="remember_me" type="checkbox" value="1"><span>Remember Me</span></label>' +
      '<div id="authNotice" class="form-message" aria-live="polite"></div>' +
      '<button class="blue-button" type="submit">Register</button></form></section></div>';
  }

  function profilePanel() {
    var user = state.currentUser || {};
    return '<div class="auth-grid"><section class="account-panel"><h2>Profile</h2>' +
      '<form id="profileForm"><div class="form-row"><label>Nickname</label><input name="nickname" type="text" value="' + escapeHTML(user.nickname || "") + '"></div>' +
      '<div class="form-row"><label>Email</label><input type="email" disabled value="' + escapeHTML(user.email || "") + '"></div>' +
      '<div class="form-row"><label>Roles</label><input type="text" disabled value="' + escapeHTML((user.roles || []).join(", ")) + '"></div>' +
      '<div id="profileNotice" class="form-message" aria-live="polite"></div>' +
      '<button class="blue-button" type="submit">Save</button></form></section>' +
      '<section class="account-panel"><h2>Activity</h2><ul class="setting-list"><li><span>Downloads</span><span>' + state.downloads.length + '</span></li><li><span>2FA</span><span>' + (user.two_factor_enabled ? "Enabled" : "Off") + '</span></li></ul></section></div>';
  }

  function securityPanel() {
    return '<div class="auth-grid"><section class="account-panel"><h2>Two-Factor Authentication</h2>' +
      '<ul class="setting-list"><li><span>Status</span><span>' + (state.currentUser && state.currentUser.two_factor_enabled ? "Enabled" : "Off") + '</span></li></ul>' +
      '<button class="metal-button" id="setup2FAButton" type="button">Setup</button>' +
      '<form id="verify2FAForm" class="inline-form"><div class="form-row"><label>Code</label><input name="totp_code" inputmode="numeric"></div><div class="filter-bar"><button class="blue-button" type="submit">Verify</button><button class="metal-button" id="disable2FAButton" type="button">Disable</button></div></form>' +
      '<div id="twoFactorNotice" class="form-message"></div></section>' +
      '<section class="account-panel"><h2>Active Sessions</h2><div id="sessionsList" class="loading">Loading...</div></section></div>';
  }

  function legacyPasswordsPanel() {
    return '<div class="auth-grid"><section class="account-panel"><h2>Legacy Passwords</h2>' +
      '<form id="legacyPasswordForm" class="inline-form"><div class="form-row"><label>Name</label><input name="name" type="text" value="Legacy Mac"></div><button class="blue-button" type="submit">Create</button></form>' +
      '<div id="legacyTokenNotice" class="notice" hidden></div><div id="legacyPasswordsList" class="loading">Loading...</div></section>' +
      '<section class="account-panel"><h2>Authorized Devices</h2><div id="legacyDevicesList" class="loading">Loading...</div></section></div>';
  }

  function bindAuthForms() {
    var login = document.getElementById("loginForm");
    var twoFactorLogin = document.getElementById("twoFactorLoginForm");
    var cancelTwoFactorLogin = document.getElementById("cancel2FALoginButton");
    var register = document.getElementById("registerForm");
    var profile = document.getElementById("profileForm");
    var setup2FA = document.getElementById("setup2FAButton");
    var verify2FA = document.getElementById("verify2FAForm");
    var disable2FA = document.getElementById("disable2FAButton");
    var legacyPassword = document.getElementById("legacyPasswordForm");
    if (login) {
      login.addEventListener("submit", function (event) {
        event.preventDefault();
        postLogin(login);
      });
    }
    if (twoFactorLogin) {
      twoFactorLogin.addEventListener("submit", function (event) {
        event.preventDefault();
        postTwoFactorLogin(twoFactorLogin);
      });
    }
    if (cancelTwoFactorLogin) {
      cancelTwoFactorLogin.addEventListener("click", function () {
        state.pendingTwoFactorLogin = null;
        renderAccount("login");
      });
    }
    if (register) {
      register.addEventListener("submit", function (event) {
        event.preventDefault();
        postAuth("/auth/register", register);
      });
    }
    if (profile) {
      profile.addEventListener("submit", function (event) {
        event.preventDefault();
        var data = formJSON(profile);
        api("/me", {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(data)
        }).then(function (payload) {
          state.currentUser = payload.user;
          document.getElementById("profileNotice").textContent = "Saved";
          renderSidebar();
        }).catch(function () {
          document.getElementById("profileNotice").textContent = "Save failed";
        });
      });
    }
    if (setup2FA) {
      setup2FA.addEventListener("click", function () {
        api("/auth/2fa/setup", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }).then(function (payload) {
          document.getElementById("twoFactorNotice").textContent = "Secret: " + payload.secret;
        }).catch(function () {
          document.getElementById("twoFactorNotice").textContent = "Setup failed";
        });
      });
    }
    if (verify2FA) {
      verify2FA.addEventListener("submit", function (event) {
        event.preventDefault();
        api("/auth/2fa/verify", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(formJSON(verify2FA)) }).then(function () {
          document.getElementById("twoFactorNotice").textContent = "Enabled";
          return loadCurrentUser();
        }).then(render).catch(function () {
          document.getElementById("twoFactorNotice").textContent = "Verification failed";
        });
      });
    }
    if (disable2FA) {
      disable2FA.addEventListener("click", function () {
        api("/auth/2fa/disable", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(formJSON(verify2FA)) }).then(function () {
          document.getElementById("twoFactorNotice").textContent = "Disabled";
          return loadCurrentUser();
        }).then(render).catch(function () {
          document.getElementById("twoFactorNotice").textContent = "Disable failed";
        });
      });
    }
    if (legacyPassword) {
      legacyPassword.addEventListener("submit", function (event) {
        event.preventDefault();
        api("/me/legacy-passwords", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(formJSON(legacyPassword)) }).then(function (payload) {
          var notice = document.getElementById("legacyTokenNotice");
          notice.hidden = false;
          notice.textContent = "New token: " + payload.token;
          loadLegacyData();
        }).catch(function () {
          var notice = document.getElementById("legacyTokenNotice");
          notice.hidden = false;
          notice.textContent = "Create failed";
        });
      });
    }
  }

  function postLogin(form) {
    var notice = document.getElementById("authNotice");
    var data = authPayloadFromForm(form);
    state.pendingTwoFactorLogin = null;
    submitAuth("/auth/login", data, data.remember_me).catch(function (error) {
      if (error && error.message === "two_factor_required") {
        state.pendingTwoFactorLogin = data;
        main.innerHTML = twoFactorLoginForm(data.email);
        bindAuthForms();
        var input = main.querySelector('[name="totp_code"]');
        if (input) {
          input.focus();
        }
        setStatus("Showing: Account", "Two-Factor Authentication", "");
        return;
      }
      notice.textContent = "Sign in failed";
    });
  }

  function postTwoFactorLogin(form) {
    var notice = document.getElementById("twoFactorLoginNotice");
    var pending = state.pendingTwoFactorLogin;
    if (!pending) {
      renderAccount("login");
      return;
    }
    var data = Object.assign({}, pending, {
      totp_code: formJSON(form).totp_code
    });
    submitAuth("/auth/login", data, pending.remember_me).then(function () {
      state.pendingTwoFactorLogin = null;
    }).catch(function () {
      notice.textContent = "Verification failed";
    });
  }

  function postAuth(path, form) {
    var notice = document.getElementById("authNotice");
    var data = authPayloadFromForm(form);
    submitAuth(path, data, data.remember_me).catch(function () {
      notice.textContent = path === "/auth/register" ? "Registration failed" : "Sign in failed";
    });
  }

  function authPayloadFromForm(form) {
    var data = {};
    var remember = !!(form.querySelector('[name="remember_me"]') && form.querySelector('[name="remember_me"]').checked);
    Array.from(new FormData(form).entries()).forEach(function (pair) {
      if (pair[0] !== "remember_me") {
        data[pair[0]] = pair[1];
      }
    });
    data.remember_me = remember;
    return data;
  }

  function submitAuth(path, data, remember) {
    return api(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data)
    }).then(function (payload) {
      rememberAuthSession(payload, remember);
      return loadCurrentUser();
    }).then(function () {
      routeTo("profile");
    });
  }

  function rememberAuthSession(payload, remember) {
    var session = payload && typeof payload.session === "object" ? payload.session : {};
    var token = payload && (payload.session_token || payload.session_id || payload.access_token || payload.token || session.token || session.id);
    if (!token) {
      return;
    }
    var secure = window.location.protocol === "https:" ? "; Secure" : "";
    var maxAge = remember ? "; Max-Age=2592000" : "";
    document.cookie = "legacystore_session=" + encodeURIComponent(token) + "; Path=/; SameSite=Lax" + secure + maxAge;
  }

  function formJSON(form) {
    var data = {};
    Array.from(new FormData(form).entries()).forEach(function (pair) {
      data[pair[0]] = pair[1];
    });
    return data;
  }

  function loadSecurityData() {
    var target = document.getElementById("sessionsList");
    if (!target) {
      return;
    }
    api("/me/sessions").then(function (payload) {
      var sessions = payload.sessions || [];
      target.innerHTML = sessions.length ? '<ul class="setting-list">' + sessions.map(function (session) {
        return '<li><span>' + escapeHTML(session.device_name || "Browser") + '<br><small>' + escapeHTML(session.last_seen_at || "") + '</small></span>' +
          '<button class="metal-button small-action" data-session-id="' + escapeHTML(session.id) + '" type="button">Revoke</button></li>';
      }).join("") + "</ul>" : '<div class="empty-state compact">No active sessions</div>';
      target.querySelectorAll("[data-session-id]").forEach(function (button) {
        button.addEventListener("click", function () {
          api("/me/sessions/" + encodeURIComponent(button.getAttribute("data-session-id")), { method: "DELETE" }).then(loadSecurityData);
        });
      });
    }).catch(function () {
      target.innerHTML = '<div class="empty-state compact">Unable to load sessions</div>';
    });
  }

  function loadLegacyData() {
    var passwordTarget = document.getElementById("legacyPasswordsList");
    var deviceTarget = document.getElementById("legacyDevicesList");
    if (passwordTarget) {
      api("/me/legacy-passwords").then(function (payload) {
        var items = payload.legacy_passwords || [];
        passwordTarget.innerHTML = items.length ? '<ul class="setting-list">' + items.map(function (item) {
          return '<li><span>' + escapeHTML(item.name) + '<br><small>' + escapeHTML(item.prefix) + '</small></span>' +
            '<span><button class="metal-button small-action" data-reset-token="' + escapeHTML(item.id) + '" type="button">Reset</button> ' +
            '<button class="metal-button small-action" data-delete-token="' + escapeHTML(item.id) + '" type="button">Revoke</button></span></li>';
        }).join("") + "</ul>" : '<div class="empty-state compact">No legacy passwords</div>';
        passwordTarget.querySelectorAll("[data-reset-token]").forEach(function (button) {
          button.addEventListener("click", function () {
            api("/me/legacy-passwords/" + encodeURIComponent(button.getAttribute("data-reset-token")) + "/reset", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }).then(function (payload) {
              var notice = document.getElementById("legacyTokenNotice");
              notice.hidden = false;
              notice.textContent = "New token: " + payload.token;
              loadLegacyData();
            });
          });
        });
        passwordTarget.querySelectorAll("[data-delete-token]").forEach(function (button) {
          button.addEventListener("click", function () {
            api("/me/legacy-passwords/" + encodeURIComponent(button.getAttribute("data-delete-token")), { method: "DELETE" }).then(loadLegacyData);
          });
        });
      }).catch(function () {
        passwordTarget.innerHTML = '<div class="empty-state compact">Unable to load legacy passwords</div>';
      });
    }
    if (deviceTarget) {
      api("/me/devices").then(function (payload) {
        var devices = payload.devices || [];
        deviceTarget.innerHTML = devices.length ? '<ul class="setting-list">' + devices.map(function (device) {
          return '<li><span>' + escapeHTML(device.device_name || "Legacy Mac") + '<br><small>' + escapeHTML(device.last_seen_at || "") + '</small></span>' +
            '<button class="metal-button small-action" data-device-id="' + escapeHTML(device.id) + '" type="button">Revoke</button></li>';
        }).join("") + "</ul>" : '<div class="empty-state compact">No authorized devices</div>';
        deviceTarget.querySelectorAll("[data-device-id]").forEach(function (button) {
          button.addEventListener("click", function () {
            api("/me/devices/" + encodeURIComponent(button.getAttribute("data-device-id")), { method: "DELETE" }).then(loadLegacyData);
          });
        });
      }).catch(function () {
        deviceTarget.innerHTML = '<div class="empty-state compact">Unable to load devices</div>';
      });
    }
  }

  function renderAdmin(route) {
    var labels = {
      uploads: "Uploads",
      moderation: "Moderation Queue",
      "admin-dashboard": "Admin Dashboard",
      users: "Users",
      "admin-apps": "Apps",
      "admin-versions": "Versions",
      "admin-artifacts": "Artifacts",
      "admin-reviews": "Reviews",
      "audit-log": "Audit Log",
      settings: "Settings"
    };
    var label = labels[route] || "Admin";
    main.innerHTML = '<div class="view-title"><h1>' + escapeHTML(label) + '</h1></div><section class="account-panel"><div id="adminContent" class="loading">Loading...</div></section>';
    setStatus("Showing: Administration", label, "Access: Restricted");
    renderSidebar();
    loadAdmin(route);
  }

  function loadAdmin(route) {
    var target = document.getElementById("adminContent");
    if (!target) {
      return;
    }
    if (route === "admin-dashboard") {
      api("/admin/dashboard").then(function (payload) {
        var rows = Object.keys(payload.dashboard || {}).sort().map(function (key) {
          return '<li><span>' + escapeHTML(key) + '</span><span>' + escapeHTML(payload.dashboard[key]) + '</span></li>';
        }).join("");
        target.innerHTML = '<ul class="setting-list">' + rows + "</ul>";
      }).catch(adminError);
      return;
    }
    if (route === "users") {
      api("/admin/users").then(function (payload) {
        target.innerHTML = '<ul class="setting-list">' + (payload.users || []).map(function (user) {
          return '<li><span>' + escapeHTML(user.email) + '<br><small>' + escapeHTML((user.roles || []).join(", ")) + '</small></span><span>' + escapeHTML(user.status) + '</span></li>';
        }).join("") + "</ul>";
      }).catch(adminError);
      return;
    }
    if (route === "admin-apps" || route === "uploads") {
      api("/admin/apps?limit=100").then(function (payload) {
        target.innerHTML = '<ul class="setting-list">' + (payload.apps || []).map(function (app) {
          return '<li><span>' + escapeHTML(app.name) + '<br><small>' + escapeHTML(app.slug) + '</small></span><span>' + escapeHTML(app.moderation_status) + '</span></li>';
        }).join("") + "</ul>";
      }).catch(adminError);
      return;
    }
    if (route === "moderation") {
      api("/admin/moderation").then(function (payload) {
        target.innerHTML = '<ul class="setting-list">' + (payload.items || []).map(function (item) {
          return '<li><span>' + escapeHTML(item.entity_type) + " #" + escapeHTML(item.entity_id) + '<br><small>' + escapeHTML(item.created_at) + '</small></span>' +
            '<span><button class="metal-button small-action" data-approve="' + escapeHTML(item.id) + '" type="button">Approve</button> ' +
            '<button class="metal-button small-action" data-reject="' + escapeHTML(item.id) + '" type="button">Reject</button></span></li>';
        }).join("") + "</ul>";
        target.querySelectorAll("[data-approve]").forEach(function (button) {
          button.addEventListener("click", function () {
            api("/admin/moderation/" + encodeURIComponent(button.getAttribute("data-approve")) + "/approve", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }).then(function () { loadAdmin("moderation"); });
          });
        });
        target.querySelectorAll("[data-reject]").forEach(function (button) {
          button.addEventListener("click", function () {
            api("/admin/moderation/" + encodeURIComponent(button.getAttribute("data-reject")) + "/reject", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }).then(function () { loadAdmin("moderation"); });
          });
        });
      }).catch(adminError);
      return;
    }
    if (route === "audit-log") {
      api("/admin/audit-log").then(function (payload) {
        target.innerHTML = '<ul class="setting-list">' + (payload.items || []).map(function (item) {
          return '<li><span>' + escapeHTML(item.action) + '<br><small>' + escapeHTML(item.created_at) + '</small></span><span>' + escapeHTML(item.entity_type || "") + " " + escapeHTML(item.entity_id || "") + '</span></li>';
        }).join("") + "</ul>";
      }).catch(adminError);
      return;
    }
    target.innerHTML = '<div class="empty-state compact">This section uses the admin API and will be filled as records are created.</div>';

    function adminError() {
      target.innerHTML = '<div class="empty-state compact">Unable to load admin data</div>';
    }
  }

  function renderError(error) {
    main.innerHTML = '<div class="empty-state">Error: ' + escapeHTML(error.message || error) + "</div>";
  }

  function render() {
    var parsed = parseRoute();
    state.route = parsed.route;
    main.classList.remove("is-auth");
    setActiveTabs();
    if (state.route === "home") {
      renderHome();
      return;
    }
    if (state.route === "catalog") {
      renderCatalog("");
      return;
    }
    if (state.route.indexOf("category/") === 0) {
      renderCatalog(state.route.split("/")[1]);
      return;
    }
    if (state.route === "categories") {
      renderCategories();
      return;
    }
    if (state.route === "search") {
      renderSearch(parsed.params.get("q") || "");
      return;
    }
    if (state.route.indexOf("app/") === 0) {
      renderApp(state.route.split("/")[1]);
      return;
    }
    if (state.route === "downloads") {
      renderDownloads();
      return;
    }
    if (state.route === "updates") {
      renderUpdates();
      return;
    }
    if (state.route === "top-charts") {
      renderTopCharts();
      return;
    }
    if (["profile", "security", "legacy-passwords"].indexOf(state.route) >= 0 && !state.currentUser) {
      routeTo("login");
      return;
    }
    if (["login", "register", "profile", "security", "legacy-passwords"].indexOf(state.route) >= 0) {
      renderAccount(state.route);
      return;
    }
    if (labelsContains(adminRoutes, state.route)) {
      if (state.showAdmin) {
        renderAdmin(state.route);
      } else {
        routeTo("login");
      }
      return;
    }
    routeTo("home");
  }

  function labelsContains(items, route) {
    return items.some(function (item) { return item[0] === route; });
  }

  document.querySelectorAll(".toolbar-tab").forEach(function (button) {
    button.addEventListener("click", function () {
      routeTo(button.getAttribute("data-route"));
    });
  });

  document.getElementById("backButton").addEventListener("click", function () {
    history.back();
  });

  document.getElementById("forwardButton").addEventListener("click", function () {
    history.forward();
  });

  searchForm.addEventListener("submit", function (event) {
    event.preventDefault();
    var query = searchInput.value.trim();
    if (query) {
      routeTo("search?q=" + encodeURIComponent(query));
    }
  });

  window.addEventListener("hashchange", render);

  function loadCurrentUser() {
    return api("/me").then(function (payload) {
      state.currentUser = payload.user || payload;
      state.currentSession = payload.session || null;
      state.showAdmin = hasAdminAccess(state.currentUser);
    }).catch(function () {
      state.currentUser = null;
      state.currentSession = null;
      state.showAdmin = false;
    });
  }

  Promise.all([
    api("/categories").then(function (payload) {
      state.categories = payload.categories || [];
    }),
    loadCurrentUser()
  ]).then(function () {
    renderSidebar();
    render();
  }).catch(renderError);
})();
