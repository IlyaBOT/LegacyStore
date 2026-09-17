(function () {
  "use strict";

  var state = {
    categories: [],
    apps: [],
    route: "home",
    routeParams: new URLSearchParams(),
    os: "10.9",
    arch: "x86_64",
    category: "",
    currentUser: null,
    currentSession: null,
    pendingTwoFactorLogin: null,
    twoFactorSetup: null,
    recoveryCodes: [],
    uploadContext: { appId: "", versionId: "", file: null, inspection: null, iconDataURL: "" }
  };

  var main = document.getElementById("main");
  var sidebar = document.getElementById("sidebar");
  var searchForm = document.getElementById("searchForm");
  var searchInput = document.getElementById("searchInput");

  var osVersions = [
    "All Versions", "10.15", "10.14", "10.13", "10.12", "10.11", "10.10",
    "10.9", "10.8", "10.7", "10.6", "10.5", "10.4"
  ];

  var osReleaseNames = {
    "10.4": "Tiger",
    "10.5": "Leopard",
    "10.6": "Snow Leopard",
    "10.7": "Lion",
    "10.8": "Mountain Lion",
    "10.9": "Mavericks",
    "10.10": "Yosemite",
    "10.11": "El Capitan",
    "10.12": "Sierra",
    "10.13": "High Sierra",
    "10.14": "Mojave",
    "10.15": "Catalina"
  };

  var appleCategories = [
    { slug: "books", name: "Books" },
    { slug: "business", name: "Business" },
    { slug: "developer-tools", name: "Developer Tools" },
    { slug: "education", name: "Education" },
    { slug: "entertainment", name: "Entertainment" },
    { slug: "finance", name: "Finance" },
    { slug: "food-drink", name: "Food & Drink" },
    { slug: "games", name: "Games" },
    { slug: "graphics-design", name: "Graphics & Design" },
    { slug: "health-fitness", name: "Health & Fitness" },
    { slug: "lifestyle", name: "Lifestyle" },
    { slug: "magazines-newspapers", name: "Magazines & Newspapers" },
    { slug: "medical", name: "Medical" },
    { slug: "music", name: "Music" },
    { slug: "navigation", name: "Navigation" },
    { slug: "news", name: "News" },
    { slug: "photo-video", name: "Photo & Video" },
    { slug: "productivity", name: "Productivity" },
    { slug: "reference", name: "Reference" },
    { slug: "safari-extensions", name: "Safari Extensions" },
    { slug: "shopping", name: "Shopping" },
    { slug: "social-networking", name: "Social Networking" },
    { slug: "sports", name: "Sports" },
    { slug: "travel", name: "Travel" },
    { slug: "utilities", name: "Utilities" },
    { slug: "weather", name: "Weather" }
  ];

  function normalizeAppleCategories(serverCategories) {
    var bySlug = {};
    (serverCategories || []).forEach(function (category) {
      bySlug[category.slug] = category;
    });
    return appleCategories.map(function (category, index) {
      var server = bySlug[category.slug] || {};
      return {
        slug: category.slug,
        name: server.name || category.name,
        sort_order: server.sort_order || ((index + 1) * 10)
      };
    });
  }

  var iconPaths = {
    account: '<circle cx="12" cy="8" r="3.2"/><path d="M5.5 20c.7-4.2 3-6.3 6.5-6.3s5.8 2.1 6.5 6.3"/>',
    security: '<rect x="5" y="10" width="14" height="10" rx="2"/><path d="M8 10V7.5a4 4 0 0 1 8 0V10"/>',
    key: '<circle cx="8" cy="12" r="4"/><path d="m12 12 7-7m-2 2 2 2m-4 0 2 2"/>',
    admin: '<path d="M12 3 4.5 6v5.3c0 4.8 3 7.8 7.5 9.7 4.5-1.9 7.5-4.9 7.5-9.7V6L12 3Z"/><path d="m9 12 2 2 4-4"/>',
    upload: '<path d="M12 16V5m0 0-4 4m4-4 4 4M5 19h14"/>',
    moderation: '<path d="M5 4h14v16H5z"/><path d="M8 8h8M8 12h8M8 16h5"/>',
    users: '<circle cx="9" cy="8" r="3"/><circle cx="17" cy="10" r="2.2"/><path d="M3.5 20c.5-4.1 2.4-6 5.5-6s5 1.9 5.5 6M14 15c2.9-.6 5 .8 6 4"/>',
    audit: '<circle cx="12" cy="12" r="8"/><path d="M12 7v5l3 2"/>',
    category: '<rect x="4" y="4" width="6" height="6" rx="1"/><rect x="14" y="4" width="6" height="6" rx="1"/><rect x="4" y="14" width="6" height="6" rx="1"/><rect x="14" y="14" width="6" height="6" rx="1"/>',
    macos: '<rect x="4" y="5" width="16" height="11" rx="1.5"/><path d="M8 20h8M10 16v4m4-4v4"/>',
    logout: '<path d="M10 4H5v16h5M14 8l4 4-4 4m4-4H9"/>',
    recovery: '<path d="M4 12a8 8 0 1 0 2.3-5.7L4 8.5M4 4v4.5h4.5"/><path d="M12 8v4l2.5 2.5"/>'
  };

  function api(path, options) {
    var requestOptions = Object.assign({ credentials: "same-origin" }, options || {});
    var headers = new Headers(requestOptions.headers || {});
    if (!headers.has("Accept")) {
      headers.set("Accept", "application/json");
    }
    requestOptions.headers = headers;
    return fetch("/api/v1" + path, requestOptions).then(function (response) {
      return response.text().then(function (text) {
        var payload = {};
        if (text) {
          try {
            payload = JSON.parse(text);
          } catch (parseError) {
            var malformed = new Error("invalid_server_response");
            malformed.status = response.status;
            throw malformed;
          }
        }
        if (!response.ok) {
          var error = new Error(payload.error || response.statusText || "request_failed");
          error.status = response.status;
          error.payload = payload;
          throw error;
        }
        return payload;
      });
    });
  }

  function jsonRequest(method, path, data) {
    return api(path, {
      method: method,
      headers: { "Content-Type": "application/json" },
      body: data == null ? "{}" : JSON.stringify(data)
    });
  }

  function escapeHTML(value) {
    return String(value == null ? "" : value)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  function routeTo(route) {
    window.location.hash = route;
  }

  function parseRoute() {
    if (window.location.pathname === "/account/recovery") {
      return { route: "recovery", params: new URLSearchParams(window.location.search) };
    }
    var hash = window.location.hash.replace(/^#/, "") || "home";
    var splitAt = hash.indexOf("?");
    var route = splitAt >= 0 ? hash.slice(0, splitAt) : hash;
    var query = splitAt >= 0 ? hash.slice(splitAt + 1) : "";
    return { route: route || "home", params: new URLSearchParams(query) };
  }

  function svgIcon(name) {
    return '<span class="source-icon source-icon-' + escapeHTML(name) + '"><svg viewBox="0 0 24 24" aria-hidden="true">' +
      (iconPaths[name] || iconPaths.category) + "</svg></span>";
  }

  function rolesOf(user) {
    return user && Array.isArray(user.roles) ? user.roles : [];
  }

  function hasRole(user) {
    var roles = rolesOf(user);
    for (var i = 1; i < arguments.length; i += 1) {
      if (roles.indexOf(arguments[i]) >= 0) {
        return true;
      }
    }
    return false;
  }

  function canUpload(user) {
    return hasRole(user, "trusted", "moder", "admin");
  }

  function canModerate(user) {
    return hasRole(user, "moder", "admin");
  }

  function isAdmin(user) {
    return hasRole(user, "admin");
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

  function sourceButton(label, route, icon, active, badge) {
    return '<button class="source-item' + (active ? " is-active" : "") + '" data-route="' + escapeHTML(route) + '" type="button">' +
      svgIcon(icon || "category") + '<span>' + escapeHTML(label) + "</span>" +
      (badge != null ? '<span class="source-badge">' + escapeHTML(badge) + "</span>" : "") +
      "</button>";
  }

  function renderSidebar() {
    var accountRoutes = state.currentUser ? [
      ["profile", "Profile", "account"],
      ["security", "Security", "security"],
      ["legacy-passwords", "Legacy Passwords", "key"],
      ["logout", "Sign Out", "logout"]
    ] : [
      ["login", "Sign In", "account"],
      ["register", "Register", "account"],
      ["recovery", "Recover Account", "recovery"]
    ];

    var managementRoutes = [];
    if (canUpload(state.currentUser)) {
      managementRoutes.push(["uploads", "Uploads", "upload"]);
    }
    if (canModerate(state.currentUser)) {
      managementRoutes.push(["moderation", "Moderation Queue", "moderation"]);
      managementRoutes.push(["admin-apps", "Apps", "category"]);
      managementRoutes.push(["admin-dashboard", "Dashboard", "admin"]);
    }
    if (isAdmin(state.currentUser)) {
      managementRoutes.push(["users", "Users", "users"]);
      managementRoutes.push(["audit-log", "Audit Log", "audit"]);
    }

    var categoryItems = state.categories.map(function (category) {
      return sourceButton(category.name, "category/" + category.slug, "category", state.category === category.slug);
    }).join("");

    var osItems = osVersions.map(function (version) {
      var value = version === "All Versions" ? "" : version;
      var active = state.os === value;
      return '<button class="source-item' + (active ? " is-active" : "") + '" data-os="' + escapeHTML(value) + '" type="button">' +
        svgIcon("macos") + '<span>' + escapeHTML(version === "All Versions" ? "All Versions" : "macOS " + version) + "</span></button>";
    }).join("");

    sidebar.innerHTML =
      '<section class="source-section"><div class="source-heading">Account</div>' +
      accountRoutes.map(function (item) { return sourceButton(item[1], item[0], item[2], state.route === item[0]); }).join("") +
      "</section>" +
      (managementRoutes.length ? '<section class="source-section"><div class="source-heading">Management</div>' +
        managementRoutes.map(function (item) { return sourceButton(item[1], item[0], item[2], state.route === item[0]); }).join("") + "</section>" : "") +
      '<section class="source-section"><div class="source-heading">Categories</div>' +
      sourceButton("All Categories", "categories", "category", state.route === "categories") +
      sourceButton("All Software", "catalog", "category", state.route === "catalog" && !state.category) +
      categoryItems + "</section>" +
      '<section class="source-section"><div class="source-heading">macOS Versions</div>' + osItems + "</section>";

    sidebar.querySelectorAll("[data-route]").forEach(function (button) {
      button.addEventListener("click", function () { routeTo(button.getAttribute("data-route")); });
    });
    sidebar.querySelectorAll("[data-os]").forEach(function (button) {
      button.addEventListener("click", function () {
        state.os = button.getAttribute("data-os") || "";
        render();
      });
    });
  }

  function appInitials(app) {
    return (app.name || app.slug || "LS").split(/\s+/).map(function (part) { return part.charAt(0); }).join("").slice(0, 2).toUpperCase();
  }

  function appIconHTML(app, small) {
    var blocked = isBlockedApp(app);
    var icon = app.icon || app.icon_url || "";
    return '<div class="app-icon' + (small ? " small" : "") + (blocked ? " is-incompatible" : "") + '">' +
      '<span class="app-icon-initials">' + escapeHTML(appInitials(app)) + "</span>" +
      (icon ? '<img class="app-icon-image" data-app-icon-img src="' + escapeHTML(icon) + '" alt="">' : "") +
      "</div>";
  }

  function bindImageFallbacks(root) {
    (root || document).querySelectorAll("[data-app-icon-img]").forEach(function (image) {
      image.addEventListener("error", function () { image.classList.add("is-broken"); }, { once: true });
      if (image.complete && image.naturalWidth === 0) {
        image.classList.add("is-broken");
      }
    });
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
      return '<span class="badge' + (index === 0 ? " gray" : "") + '">' + escapeHTML(item) + "</span>";
    }).join("");
  }

  function isBlockedApp(app) {
    return !!(app && app.compatibility && app.compatibility.status === "blocked");
  }

  function compatibilityPill(compatibility) {
    if (!compatibility || compatibility.status === "recommended" || compatibility.status === "compatible") {
      return "";
    }
    var label = compatibility.label || compatibility.status || "Unknown";
    return '<span class="compat-pill">' + escapeHTML(label) + "</span>";
  }

  function queryTarget() {
    return "os=" + encodeURIComponent(state.os || "10.9") + "&arch=" + encodeURIComponent(state.arch) + "&os_series=1";
  }

  function loadApps(category, limit) {
    var path = "/apps?" + queryTarget() + "&limit=" + encodeURIComponent(limit || 24) + "&page=1";
    if (category) {
      path += "&category=" + encodeURIComponent(category);
    }
    return api(path).then(function (payload) { return payload.apps || []; });
  }

  function appCard(app) {
    var blocked = isBlockedApp(app);
    return '<article class="app-card' + (blocked ? " is-incompatible" : "") + '">' +
      appIconHTML(app, false) + '<h3>' + escapeHTML(app.name) + '</h3><div class="app-meta">' + escapeHTML(app.category || "") + "</div>" +
      '<div class="rating-row">' + ratingDots(app.rating) + '<span class="muted">(' + escapeHTML(app.rating_count || 0) + ")</span></div>" +
      '<div class="badge-row">' + badges(app.arch_badges) + compatibilityPill(app.compatibility) + "</div>" +
      '<button class="metal-button" data-route="app/' + escapeHTML(app.slug) + '" type="button">View</button></article>';
  }

  function appRow(app) {
    var blocked = isBlockedApp(app);
    return '<article class="app-row' + (blocked ? " is-incompatible" : "") + '">' + appIconHTML(app, true) +
      '<div><div class="list-name">' + escapeHTML(app.name) + '</div><div class="muted">' + escapeHTML(app.summary || "") + "</div></div>" +
      '<div><strong>' + escapeHTML(app.category || "") + '</strong><div class="muted">Version ' + escapeHTML(app.recommended_version || "") + "</div></div>" +
      '<div><div class="badge-row">' + badges(app.arch_badges) + compatibilityPill(app.compatibility) + "</div></div>" +
      '<div><button class="metal-button" data-route="app/' + escapeHTML(app.slug) + '" type="button">View</button></div></article>';
  }

  function bindRouteButtons(root) {
    root.querySelectorAll("[data-route]").forEach(function (button) {
      button.addEventListener("click", function () { routeTo(button.getAttribute("data-route")); });
    });
  }

  function categoryName(slug) {
    var found = state.categories.find(function (category) { return category.slug === slug; });
    return found ? found.name : "";
  }

  function filtersHTML() {
    return '<div class="filter-bar"><select id="osFilter" aria-label="macOS version">' +
      osVersions.map(function (version) {
        var value = version === "All Versions" ? "" : version;
        return '<option value="' + escapeHTML(value) + '"' + (state.os === value ? " selected" : "") + ">" + escapeHTML(version) + "</option>";
      }).join("") + '</select><select id="archFilter" aria-label="Architecture">' +
      '<option value="x86_64"' + (state.arch === "x86_64" ? " selected" : "") + '>x86_64</option>' +
      '<option value="i386"' + (state.arch === "i386" ? " selected" : "") + '>i386</option></select></div>';
  }

  function bindFilters() {
    var osFilter = document.getElementById("osFilter");
    var archFilter = document.getElementById("archFilter");
    if (osFilter) {
      osFilter.addEventListener("change", function () { state.os = osFilter.value; render(); });
    }
    if (archFilter) {
      archFilter.addEventListener("change", function () { state.arch = archFilter.value; render(); });
    }
  }

  function panel(title, route, body) {
    return '<section class="section-panel"><div class="section-header"><h2>' + escapeHTML(title) + '</h2>' +
      '<button class="link-button" data-route="' + escapeHTML(route) + '" type="button">See All</button></div>' +
      '<div class="app-strip">' + (body || '<div class="empty-state">No items</div>') + "</div></section>";
  }

  function renderHome() {
    state.category = "";
    main.innerHTML = '<div class="loading">Loading...</div>';
    Promise.all([loadApps("", 12), loadApps("graphics-design", 6)]).then(function (sets) {
      state.apps = sets[0];
      var featured = state.apps.find(function (app) { return app.slug === "pixelmator"; }) || state.apps[0] || {};
      main.innerHTML = '<section class="hero"><div class="hero-content"><h1>' + escapeHTML(featured.name || "LegacyStore") + "</h1><p>" +
        escapeHTML(featured.summary || "Classic software catalog for old Intel Macs.") + '</p><button class="blue-button" data-route="app/' +
        escapeHTML(featured.slug || "pixelmator") + '" type="button">View Application</button></div></section>' +
        panel("New and Noteworthy", "catalog", sets[0].slice(0, 6).map(appCard).join("")) +
        panel("Graphics & Design", "category/graphics-design", sets[1].slice(0, 6).map(appCard).join(""));
      bindRouteButtons(main);
      bindImageFallbacks(main);
      setStatus("Showing: Featured", state.apps.length + " items", "Sort By: Featured");
      renderSidebar();
    }).catch(renderError);
  }

  function renderCatalog(category) {
    state.category = category || "";
    main.innerHTML = '<div class="loading">Loading...</div>';
    loadApps(state.category, 50).then(function (apps) {
      state.apps = apps;
      var title = categoryName(state.category) || "All Software";
      main.innerHTML = '<div class="view-title"><h1>' + escapeHTML(title) + "</h1>" + filtersHTML() + '</div><div class="app-list">' +
        (apps.length ? apps.map(appRow).join("") : '<div class="empty-state">No items</div>') + "</div>";
      bindRouteButtons(main);
      bindFilters();
      bindImageFallbacks(main);
      setStatus("Showing: " + title, apps.length + " items", "Compatibility shown for selected Mac");
      renderSidebar();
    }).catch(renderError);
  }

  function renderCategories() {
    state.category = "";
    main.innerHTML = '<div class="view-title"><h1>Categories</h1>' + filtersHTML() + '</div><div class="category-grid">' +
      state.categories.map(function (category) {
        return '<button class="category-tile" data-route="category/' + escapeHTML(category.slug) + '" type="button">' + svgIcon("category") +
          '<strong>' + escapeHTML(category.name) + "</strong></button>";
      }).join("") + "</div>";
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
      main.innerHTML = '<div class="view-title"><h1>Search</h1>' + filtersHTML() + '</div><div class="app-list">' +
        (results.length ? results.map(appRow).join("") : '<div class="empty-state">No items</div>') + "</div>";
      bindRouteButtons(main);
      bindFilters();
      bindImageFallbacks(main);
      setStatus('Search: "' + query + '"', results.length + " items", "Sort By: Relevance");
      renderSidebar();
    }).catch(renderError);
  }

  function screenshotStrip(app) {
    var screenshots = app.screenshots || [];
    if (!screenshots.length) {
      return "";
    }
    return '<div class="screenshot-strip">' + screenshots.map(function (shot) {
      return '<img src="' + escapeHTML(shot.image_url) + '" alt="' + escapeHTML(shot.caption || app.name) + '" loading="lazy">';
    }).join("") + "</div>";
  }

  function compatibilityBlock(compatibility) {
    if (!compatibility || compatibility.status !== "blocked") {
      return "";
    }
    var reasons = compatibility.reasons || [];
    return '<div class="notice warning"><strong>This product is not compatible with the selected Mac.</strong>' +
      (reasons.length ? '<ul>' + reasons.map(function (reason) { return '<li>' + escapeHTML(reason.message || reason.code) + "</li>"; }).join("") + "</ul>" : "") + "</div>";
  }

  function parseOSVersionForDisplay(raw) {
    var text = String(raw || "").trim();
    if (!text) { return null; }
    var parts = text.split(".");
    if (parts.length < 2) { return null; }
    return {
      raw: text,
      series: parts[0] + "." + parts[1],
      patch: parts.length > 2 ? Number(parts[2]) : null,
      hasPatch: parts.length > 2
    };
  }

  function osVersionDisplay(raw) {
    var parsed = parseOSVersionForDisplay(raw);
    if (!parsed) { return String(raw || ""); }
    var release = osReleaseNames[parsed.series];
    return "OS X " + parsed.raw + (release ? " (" + release + ")" : "");
  }

  function artifactSupportedSystemsLabel(artifact) {
    artifact = artifact || {};
    var minOS = artifact.min_os || "";
    var maxOS = artifact.max_supported_os || "";
    if (minOS && maxOS && minOS === maxOS) {
      return osVersionDisplay(minOS);
    }
    if (minOS && maxOS) {
      return osVersionDisplay(minOS) + " – " + osVersionDisplay(maxOS);
    }
    if (minOS) {
      return osVersionDisplay(minOS) + " or later";
    }
    if (maxOS) {
      return osVersionDisplay(maxOS) + " or earlier";
    }
    return "Not specified";
  }

  function artifactPatchRequirementNote(artifact) {
    artifact = artifact || {};
    var minOS = parseOSVersionForDisplay(artifact.min_os);
    var maxOS = parseOSVersionForDisplay(artifact.max_supported_os);
    var notes = [];

    if (minOS && maxOS && minOS.raw === maxOS.raw && (minOS.hasPatch || maxOS.hasPatch)) {
      notes.push("This package is declared for OS X " + minOS.raw + " specifically. Operation on other system releases is not guaranteed.");
      return notes.join(" ");
    }

    if (minOS && minOS.hasPatch && minOS.patch > 0) {
      var minRelease = osReleaseNames[minOS.series];
      notes.push("Note: this application requires OS X " + minOS.raw + " or later. Earlier releases of OS X " + minOS.series +
        (minRelease ? " (" + minRelease + ")" : "") + " may not launch the application.");
    }

    if (maxOS && maxOS.hasPatch && artifact.hard_block_above_max) {
      var maxRelease = osReleaseNames[maxOS.series];
      notes.push("This package is supported only through OS X " + maxOS.raw + ". Later releases of OS X " + maxOS.series +
        (maxRelease ? " (" + maxRelease + ")" : "") + " may not work.");
    }

    return notes.join(" ");
  }

  function artifactSystemRequirementsHTML(artifact) {
    if (!artifact || !artifact.id) { return ""; }
    var note = artifactPatchRequirementNote(artifact);
    return '<div class="package-system-info"><p><strong>Supported systems:</strong><br><span class="muted">' +
      escapeHTML(artifactSupportedSystemsLabel(artifact)) + "</span></p>" +
      (note ? '<p class="package-system-note">' + escapeHTML(note) + "</p>" : "") + "</div>";
  }

  function versionsTable(versions, app) {
    if (!versions || !versions.length) {
      return '<h2>Versions</h2><div class="empty-state compact">No versions</div>';
    }
    var rows = [];
    versions.forEach(function (version) {
      (version.artifacts || []).forEach(function (artifact) {
        var blocked = artifact.compatibility_status === "blocked";
        rows.push('<tr><td>' + escapeHTML(version.version) + '</td><td>' + escapeHTML(version.release_date || "") + '</td><td>' +
          escapeHTML((artifact.archs || []).join(", ")) + '</td><td>' + escapeHTML(artifactSupportedSystemsLabel(artifact)) + '</td><td>' +
          escapeHTML(artifact.compatibility_label || artifact.compatibility_status || "") + '</td><td class="actions">' +
          '<button class="metal-button small-action" data-download-artifact="' + escapeHTML(artifact.id) + '" data-download-app="' + escapeHTML(app.name) +
          '" data-download-version="' + escapeHTML(version.version) + '" type="button"' + (blocked ? " disabled" : "") + ">Download</button></td></tr>");
      });
    });
    return '<h2>Versions</h2><table class="version-table"><thead><tr><th>Version</th><th>Date</th><th>Arch</th><th>Systems</th><th>Status</th><th></th></tr></thead><tbody>' + rows.join("") + "</tbody></table>";
  }

  function renderApp(slug) {
    state.category = "";
    main.innerHTML = '<div class="loading">Loading...</div>';
    api("/apps/" + encodeURIComponent(slug) + "?" + queryTarget()).then(function (app) {
      var artifact = app.recommended_artifact || {};
      var blocked = isBlockedApp(app);
      main.innerHTML = '<section class="detail-head">' + appIconHTML(app, false) + '<div><h1>' + escapeHTML(app.name) + '</h1><div class="muted">' +
        escapeHTML(app.developer_name || "") + '</div><p>' + escapeHTML(app.summary || "") + '</p><div class="badge-row">' + badges(artifact.archs || []) +
        compatibilityPill(app.compatibility) + '</div></div><div class="detail-actions"><button class="blue-button" id="downloadButton" type="button"' +
        (!artifact.id || blocked ? " disabled" : "") + '>Download</button><button class="metal-button" data-route="catalog" type="button">Back to Catalog</button></div></section>' +
        '<div class="detail-grid"><section class="detail-copy"><h2>Description</h2><p>' + escapeHTML(app.description || "") + '</p>' + screenshotStrip(app) +
        compatibilityBlock(app.compatibility) + versionsTable(app.versions || [], app) + reviewsHTML(app.slug) + '</section><aside class="side-box"><h2>Information</h2>' +
        infoRow("Category", app.category) + infoRow("Version", artifact.version) + infoRow("Size", formatSize(artifact.size_bytes)) + infoRow("Package", artifact.package_type) +
        artifactSystemRequirementsHTML(artifact) + infoRow("Minimum macOS", artifact.min_os) + infoRow("Maximum supported", artifact.max_supported_os) +
        infoRow("SHA-256", artifact.sha256) + '</aside></div>';
      bindRouteButtons(main);
      bindImageFallbacks(main);
      bindArtifactButtons(main);
      var button = document.getElementById("downloadButton");
      if (button && artifact.id && !blocked) {
        button.addEventListener("click", function () { loadDownloadMetadata(artifact.id, app.name, artifact.version); });
      }
      loadReviews(app.slug);
      setStatus("Showing: " + app.name, "1 item", "Version: " + (artifact.version || ""));
      renderSidebar();
    }).catch(renderError);
  }

  function bindArtifactButtons(root) {
    root.querySelectorAll("[data-download-artifact]").forEach(function (button) {
      button.addEventListener("click", function () {
        loadDownloadMetadata(button.getAttribute("data-download-artifact"), button.getAttribute("data-download-app"), button.getAttribute("data-download-version"));
      });
    });
  }

  function reviewsHTML(slug) {
    var form = state.currentUser ? '<form id="reviewForm" class="review-form" data-slug="' + escapeHTML(slug) + '"><h2>Your Review</h2>' +
      '<div class="filter-bar"><select name="rating" aria-label="Rating"><option>5</option><option>4</option><option>3</option><option>2</option><option>1</option></select>' +
      '<input name="title" type="text" placeholder="Title"></div><div class="form-row"><label>Review</label><textarea name="body" required></textarea></div>' +
      '<button class="blue-button" type="submit">Post Review</button><div id="reviewNotice" class="form-message"></div></form>' : "";
    return '<section class="reviews-section">' + form + '<h2>Reviews</h2><div id="reviewsList" class="loading">Loading...</div></section>';
  }

  function loadReviews(slug) {
    var target = document.getElementById("reviewsList");
    if (!target) { return; }
    var form = document.getElementById("reviewForm");
    if (form) {
      form.addEventListener("submit", function (event) {
        event.preventDefault();
        var data = formJSON(form);
        data.rating = Number(data.rating || 5);
        jsonRequest("POST", "/apps/" + encodeURIComponent(slug) + "/reviews", data).then(function () {
          form.reset();
          return loadReviews(slug);
        }).catch(function (error) { showMessage("reviewNotice", humanError(error), true); });
      });
    }
    api("/apps/" + encodeURIComponent(slug) + "/reviews").then(function (payload) {
      var reviews = payload.reviews || [];
      target.innerHTML = reviews.length ? '<ul class="setting-list">' + reviews.map(function (review) {
        return '<li><span><strong>' + escapeHTML(review.author || "User") + '</strong> ' + ratingDots(review.rating) + '<br>' + escapeHTML(review.title || "") +
          '<br><small>' + escapeHTML(review.body || "") + '</small></span><span>' + escapeHTML(review.likes || 0) + ' likes' +
          (state.currentUser ? ' <button class="metal-button small-action" data-like-review="' + escapeHTML(review.id) + '" type="button">Like</button>' : "") + '</span></li>';
      }).join("") + "</ul>" : '<div class="empty-state compact">No reviews</div>';
      target.querySelectorAll("[data-like-review]").forEach(function (button) {
        button.addEventListener("click", function () {
          jsonRequest("POST", "/reviews/" + encodeURIComponent(button.getAttribute("data-like-review")) + "/like", {}).then(function () { loadReviews(slug); });
        });
      });
    }).catch(function () { target.innerHTML = '<div class="empty-state compact">Unable to load reviews</div>'; });
  }

  function infoRow(label, value) {
    if (value == null || value === "") { return ""; }
    return '<p><strong>' + escapeHTML(label) + ':</strong> <span class="muted mono">' + escapeHTML(value) + "</span></p>";
  }

  function formatSize(bytes) {
    bytes = Number(bytes || 0);
    if (!bytes) { return ""; }
    if (bytes >= 1024 * 1024 * 1024) { return (bytes / (1024 * 1024 * 1024)).toFixed(1) + " GB"; }
    if (bytes >= 1024 * 1024) { return (bytes / (1024 * 1024)).toFixed(1) + " MB"; }
    return (bytes / 1024).toFixed(1) + " KB";
  }

  function loadDownloadMetadata(id, appName, version) {
    api("/download/" + encodeURIComponent(id) + "?" + queryTarget()).then(function (metadata) {
      var source = metadata.download_url || metadata.external_page_url || "";
      if (!source) {
        throw new Error("download_source_unavailable");
      }
      setStatus("Download: " + (appName || metadata.file_name || "Application"), version || metadata.version || "", "Handled by browser");
      window.location.assign(source);
    }).catch(renderError);
  }

  function renderTopCharts() {
    state.category = "";
    main.innerHTML = '<div class="loading">Loading...</div>';
    loadApps("", 50).then(function (apps) {
      apps.sort(function (a, b) { return Number(b.rating_count || 0) - Number(a.rating_count || 0); });
      main.innerHTML = '<div class="view-title"><h1>Top Charts</h1>' + filtersHTML() + '</div><div class="app-list">' + apps.map(appRow).join("") + "</div>";
      bindRouteButtons(main); bindFilters(); bindImageFallbacks(main);
      setStatus("Showing: Top Charts", apps.length + " items", "Sort By: Rating");
      renderSidebar();
    }).catch(renderError);
  }

  function loginForm() {
    return '<div class="auth-window"><section class="account-panel auth-panel"><h2>Sign In</h2>' + secureContextNote() +
      '<form id="loginForm"><div class="form-row"><label>Email</label><input name="email" type="email" autocomplete="username" required></div>' +
      '<div class="form-row"><label>Password</label><input name="password" type="password" autocomplete="current-password" required></div>' +
      '<label class="check-row"><input name="remember_me" type="checkbox" value="1"><span>Remember Me</span></label><div id="authNotice" class="form-message" aria-live="polite"></div>' +
      '<button class="blue-button" type="submit">Sign In</button></form><div class="auth-links"><button class="auth-link" data-route="register" type="button">Create account</button>' +
      '<button class="auth-link" data-route="recovery" type="button">Forgot password?</button></div></section></div>';
  }

  function twoFactorLoginForm(email) {
    return '<div class="auth-window auth-window-small"><section class="account-panel auth-panel"><h2>Two-Factor Authentication</h2><p class="auth-summary">' + escapeHTML(email) + '</p>' +
      '<form id="twoFactorLoginForm"><div class="two-factor-choice"><div class="form-row"><label>Authenticator code</label><input name="totp_code" inputmode="numeric" autocomplete="one-time-code"></div>' +
      '<span class="or">or</span><div class="form-row"><label>Recovery code</label><input name="recovery_code" autocomplete="one-time-code"></div></div>' +
      '<div id="twoFactorLoginNotice" class="form-message"></div><div class="form-actions"><button class="blue-button" type="submit">Verify</button>' +
      '<button class="metal-button" id="cancel2FALoginButton" type="button">Back</button></div></form></section></div>';
  }

  function registerForm() {
    return '<div class="auth-window"><section class="account-panel auth-panel"><h2>Register</h2>' + secureContextNote() +
      '<form id="registerForm"><div class="form-row"><label>Email</label><input name="email" type="email" autocomplete="email" required></div>' +
      '<div class="form-row"><label>Nickname</label><input name="nickname" type="text" autocomplete="nickname" required></div>' +
      '<div class="form-row"><label>Password</label><input name="password" type="password" autocomplete="new-password" minlength="8" required></div>' +
      '<label class="check-row"><input name="remember_me" type="checkbox" value="1"><span>Remember Me</span></label><div id="authNotice" class="form-message"></div>' +
      '<button class="blue-button" type="submit">Register</button></form><div class="auth-links"><button class="auth-link" data-route="login" type="button">Already have an account?</button></div></section></div>';
  }

  function secureContextNote() {
    if (window.location.protocol === "https:") { return ""; }
    return '<div class="web-security-note">Account authentication requires HTTPS. Open the HTTPS web endpoint before signing in.</div>';
  }

  function profilePanel() {
    var user = state.currentUser || {};
    return '<div class="auth-grid"><section class="account-panel"><h2>Profile</h2><form id="profileForm">' +
      '<div class="form-row"><label>Nickname</label><input name="nickname" type="text" value="' + escapeHTML(user.nickname || "") + '"></div>' +
      '<div class="form-row"><label>Avatar URL</label><input name="avatar_url" type="url" value="' + escapeHTML(user.avatar_url || "") + '"></div>' +
      '<div class="form-row"><label>Email</label><input type="email" disabled value="' + escapeHTML(user.email || "") + '"></div>' +
      '<div class="form-row"><label>Roles</label><input type="text" disabled value="' + escapeHTML(rolesOf(user).join(", ")) + '"></div>' +
      '<div id="profileNotice" class="form-message"></div><button class="blue-button" type="submit">Save</button></form></section>' +
      '<section class="account-panel"><h2>Account</h2><ul class="setting-list"><li><span>Email verified</span><span>' + (user.email_verified ? "Yes" : "No") + '</span></li>' +
      '<li><span>Two-factor authentication</span><span>' + (user.two_factor_enabled ? "Enabled" : "Off") + "</span></li></ul></section></div>";
  }

  function secondFactorFields(prefix) {
    return '<div class="two-factor-choice"><div class="form-row"><label>Authenticator code</label><input name="totp_code" inputmode="numeric" autocomplete="one-time-code"></div>' +
      '<span class="or">or</span><div class="form-row"><label>Recovery code</label><input name="recovery_code" autocomplete="one-time-code"></div></div>' +
      '<div class="panel-help">Use one second factor when 2FA is enabled.</div>' + (prefix ? "" : "");
  }

  function recoveryCodesHTML() {
    if (!state.recoveryCodes.length) { return ""; }
    return '<div class="recovery-code-box"><strong>Save these recovery codes now.</strong><p class="panel-help">They are shown only in this response. Each code can be used once.</p><ul class="recovery-codes">' +
      state.recoveryCodes.map(function (code) { return '<li>' + escapeHTML(code) + "</li>"; }).join("") + "</ul></div>";
  }

  function securityPanel() {
    var enabled = !!(state.currentUser && state.currentUser.two_factor_enabled);
    var twoFactorBody;
    if (!enabled) {
      twoFactorBody = '<p class="panel-help">Setup requires your current account password. After scanning the secret, confirm with a TOTP code.</p>' +
        '<form id="setup2FAForm"><div class="form-row"><label>Current password</label><input name="current_password" type="password" autocomplete="current-password" required></div>' +
        '<div class="form-actions"><button class="metal-button" type="submit">Generate 2FA secret</button></div></form>' +
        (state.twoFactorSetup ? '<div class="secret-box"><strong>Secret</strong><div class="mono">' + escapeHTML(state.twoFactorSetup.secret || "") + '</div><div class="mono">' +
          escapeHTML(state.twoFactorSetup.otpauth_url || "") + '</div></div><form id="verify2FAForm"><div class="form-row"><label>Authenticator code</label><input name="totp_code" inputmode="numeric" autocomplete="one-time-code" required></div>' +
          '<div class="form-actions"><button class="blue-button" type="submit">Enable 2FA</button></div></form>' : "");
    } else {
      twoFactorBody = '<p class="panel-help">2FA is enabled. Recovery codes can replace a TOTP code once each.</p>' + recoveryCodesHTML() +
        '<form id="regenerateRecoveryCodesForm"><h3>Regenerate recovery codes</h3><div class="form-row"><label>Current password</label><input name="current_password" type="password" autocomplete="current-password" required></div>' +
        secondFactorFields("regenerate") + '<div class="form-actions"><button class="metal-button" type="submit">Regenerate Codes</button></div></form>' +
        '<form id="disable2FAForm"><h3>Disable 2FA</h3><div class="form-row"><label>Current password</label><input name="current_password" type="password" autocomplete="current-password" required></div>' +
        secondFactorFields("disable") + '<div class="form-actions"><button class="metal-button" type="submit">Disable 2FA</button></div></form>';
    }

    return '<div class="security-stack"><section class="account-panel"><h2>Two-Factor Authentication</h2><ul class="setting-list"><li><span>Status</span><span>' + (enabled ? "Enabled" : "Off") + "</span></li></ul>" +
      twoFactorBody + '<div id="twoFactorNotice" class="form-message"></div></section>' +
      '<section class="account-panel"><h2>Change Password</h2><form id="changePasswordForm"><div class="form-grid"><div class="form-row"><label>Current password</label><input name="current_password" type="password" autocomplete="current-password" required></div>' +
      '<div class="form-row"><label>New password</label><input name="new_password" type="password" autocomplete="new-password" minlength="8" required></div><div class="span-2">' + secondFactorFields("password") +
      '</div></div><div id="passwordNotice" class="form-message"></div><button class="blue-button" type="submit">Change Password</button></form></section>' +
      '<section class="account-panel"><h2>Change Email</h2><form id="changeEmailForm"><div class="form-grid"><div class="form-row"><label>New email</label><input name="new_email" type="email" autocomplete="email" required></div>' +
      '<div class="form-row"><label>Current password</label><input name="current_password" type="password" autocomplete="current-password" required></div><div class="span-2">' + secondFactorFields("email") +
      '</div></div><div id="emailNotice" class="form-message"></div><button class="blue-button" type="submit">Change Email</button></form></section>' +
      '<section class="account-panel"><h2>Active Sessions</h2><div id="sessionsList" class="loading">Loading...</div></section></div>';
  }

  function legacyPasswordsPanel() {
    return '<div class="auth-grid"><section class="account-panel"><h2>Legacy Passwords</h2><p class="panel-help">These credentials are limited to the legacy client and cannot manage your account.</p>' +
      '<form id="legacyPasswordForm"><div class="form-row"><label>Name</label><input name="name" type="text" value="Legacy Mac"></div><div class="form-actions"><button class="blue-button" type="submit">Create</button></div></form>' +
      '<div id="legacyTokenNotice" class="notice" hidden></div><div id="legacyPasswordsList" class="loading">Loading...</div></section>' +
      '<section class="account-panel"><h2>Authorized Devices</h2><div id="legacyDevicesList" class="loading">Loading...</div></section></div>';
  }

  function recoveryPanel(params) {
    var token = params.get("token") || "";
    if (token) {
      return '<div class="auth-window"><section class="account-panel auth-panel"><h2>Reset Password</h2>' + secureContextNote() +
        '<form id="recoveryResetForm"><input name="token" type="hidden" value="' + escapeHTML(token) + '"><div class="form-row"><label>New password</label>' +
        '<input name="new_password" type="password" autocomplete="new-password" minlength="8" required></div><div id="recoveryNotice" class="form-message"></div>' +
        '<button class="blue-button" type="submit">Reset Password</button></form></section></div>';
    }
    return '<div class="auth-window"><section class="account-panel auth-panel"><h2>Recover Account</h2>' + secureContextNote() +
      '<p class="panel-help">If the account exists, LegacyStore will send a one-time recovery link. The response does not reveal whether an email is registered.</p>' +
      '<form id="recoveryRequestForm"><div class="form-row"><label>Email</label><input name="email" type="email" autocomplete="email" required></div>' +
      '<div id="recoveryNotice" class="form-message"></div><button class="blue-button" type="submit">Send Recovery Link</button></form>' +
      '<div class="auth-links"><button class="auth-link" data-route="login" type="button">Back to sign in</button></div></section></div>';
  }

  function renderAccount(route, params) {
    var titles = { login: "Sign In", register: "Register", recovery: "Recover Account", profile: "Profile", security: "Security", "legacy-passwords": "Legacy Passwords" };
    var body = "";
    if (route === "login") { body = loginForm(); }
    if (route === "register") { body = registerForm(); }
    if (route === "recovery") { body = recoveryPanel(params || new URLSearchParams()); }
    if (route === "profile") { body = profilePanel(); }
    if (route === "security") { body = securityPanel(); }
    if (route === "legacy-passwords") { body = legacyPasswordsPanel(); }
    if (["login", "register", "recovery"].indexOf(route) >= 0) {
      main.classList.add("is-auth");
      main.innerHTML = body;
    } else {
      main.innerHTML = '<div class="view-title"><h1>' + escapeHTML(titles[route]) + "</h1></div>" + body;
    }
    bindRouteButtons(main);
    bindAuthForms();
    if (route === "security") { loadSecurityData(); }
    if (route === "legacy-passwords") { loadLegacyData(); }
    setStatus("Showing: Account", titles[route], "");
    renderSidebar();
  }

  function formJSON(form) {
    var data = {};
    Array.from(new FormData(form).entries()).forEach(function (pair) { data[pair[0]] = pair[1]; });
    return data;
  }

  function authPayloadFromForm(form) {
    var data = formJSON(form);
    data.remember_me = !!(form.querySelector('[name="remember_me"]') && form.querySelector('[name="remember_me"]').checked);
    return data;
  }

  function humanError(error) {
    var code = error && error.message ? error.message : "request_failed";
    var messages = {
      invalid_credentials: "The credentials or second factor are invalid.",
      two_factor_required: "A second factor is required.",
      email_exists: "That email address is already in use.",
      https_required: "This action requires HTTPS.",
      invalid_recovery_token: "The recovery link is invalid or expired.",
      recovery_delivery_unavailable: "Password recovery delivery is not configured on this server.",
      csrf_origin_rejected: "The request origin was rejected.",
      forbidden: "Your account does not have permission for this action.",
      unauthorized: "Please sign in again.",
      rate_limited: "Too many attempts. Try again later.",
      metadata_too_large: "Metadata file is too large.",
      plist_required: "Info.plist was not found in the dropped app bundle.",
      download_source_unavailable: "No download source is available."
    };
    return messages[code] || code.replace(/_/g, " ");
  }

  function showMessage(id, message, isError) {
    var node = document.getElementById(id);
    if (!node) { return; }
    node.textContent = message;
    node.classList.toggle("is-error", !!isError);
    node.classList.toggle("is-success", !isError && !!message);
  }

  function toastHost() {
    var host = document.getElementById("toastHost");
    if (!host) {
      host = document.createElement("div");
      host.id = "toastHost";
      host.className = "toast-host";
      host.setAttribute("aria-live", "polite");
      document.body.appendChild(host);
    }
    return host;
  }

  function showToast(message, type) {
    if (!message) { return; }
    var toast = document.createElement("div");
    toast.className = "bubble-toast " + (type || "error");
    toast.textContent = message;
    toastHost().appendChild(toast);
    window.setTimeout(function () {
      if (toast.parentNode) { toast.parentNode.removeChild(toast); }
    }, 5000);
  }

  function showInspectionWarnings(warnings) {
    warnings = warnings || [];
    warnings.slice(0, 3).forEach(function (warning) {
      showToast(warning.message || warning.code || "Unknown metadata warning", "error");
    });
    if (warnings.length > 3) {
      showToast("Ещё " + (warnings.length - 3) + " предупреждений по метаданным.", "error");
    }
  }

  function submitAuth(path, data) {
    return jsonRequest("POST", path, data).then(function () { return loadCurrentUser(); }).then(function () { routeTo("profile"); });
  }

  function postLogin(form) {
    var data = authPayloadFromForm(form);
    state.pendingTwoFactorLogin = null;
    submitAuth("/auth/login", data).catch(function (error) {
      if (error.message === "two_factor_required") {
        state.pendingTwoFactorLogin = data;
        main.innerHTML = twoFactorLoginForm(data.email);
        bindAuthForms();
        setStatus("Showing: Account", "Two-Factor Authentication", "");
        return;
      }
      showMessage("authNotice", humanError(error), true);
    });
  }

  function postTwoFactorLogin(form) {
    var pending = state.pendingTwoFactorLogin;
    if (!pending) { renderAccount("login"); return; }
    var factor = formJSON(form);
    var data = Object.assign({}, pending, { totp_code: factor.totp_code || "", recovery_code: factor.recovery_code || "" });
    if (!data.totp_code && !data.recovery_code) {
      showMessage("twoFactorLoginNotice", "Enter an authenticator code or a recovery code.", true);
      return;
    }
    submitAuth("/auth/login", data).then(function () { state.pendingTwoFactorLogin = null; }).catch(function (error) {
      showMessage("twoFactorLoginNotice", humanError(error), true);
    });
  }

  function bindAuthForms() {
    var login = document.getElementById("loginForm");
    var register = document.getElementById("registerForm");
    var twoFactorLogin = document.getElementById("twoFactorLoginForm");
    var cancel = document.getElementById("cancel2FALoginButton");
    var profile = document.getElementById("profileForm");
    var setup2FA = document.getElementById("setup2FAForm");
    var verify2FA = document.getElementById("verify2FAForm");
    var regenerate = document.getElementById("regenerateRecoveryCodesForm");
    var disable2FA = document.getElementById("disable2FAForm");
    var changePassword = document.getElementById("changePasswordForm");
    var changeEmail = document.getElementById("changeEmailForm");
    var recoveryRequest = document.getElementById("recoveryRequestForm");
    var recoveryReset = document.getElementById("recoveryResetForm");
    var legacyPassword = document.getElementById("legacyPasswordForm");

    if (login) { login.addEventListener("submit", function (event) { event.preventDefault(); postLogin(login); }); }
    if (register) {
      register.addEventListener("submit", function (event) {
        event.preventDefault();
        submitAuth("/auth/register", authPayloadFromForm(register)).catch(function (error) { showMessage("authNotice", humanError(error), true); });
      });
    }
    if (twoFactorLogin) { twoFactorLogin.addEventListener("submit", function (event) { event.preventDefault(); postTwoFactorLogin(twoFactorLogin); }); }
    if (cancel) { cancel.addEventListener("click", function () { state.pendingTwoFactorLogin = null; renderAccount("login"); }); }
    if (profile) {
      profile.addEventListener("submit", function (event) {
        event.preventDefault();
        jsonRequest("PATCH", "/me", formJSON(profile)).then(function (payload) {
          state.currentUser = payload.user;
          showMessage("profileNotice", "Saved", false);
          renderSidebar();
        }).catch(function (error) { showMessage("profileNotice", humanError(error), true); });
      });
    }
    if (setup2FA) {
      setup2FA.addEventListener("submit", function (event) {
        event.preventDefault();
        jsonRequest("POST", "/auth/2fa/setup", formJSON(setup2FA)).then(function (payload) {
          state.twoFactorSetup = payload;
          renderAccount("security");
        }).catch(function (error) { showMessage("twoFactorNotice", humanError(error), true); });
      });
    }
    if (verify2FA) {
      verify2FA.addEventListener("submit", function (event) {
        event.preventDefault();
        jsonRequest("POST", "/auth/2fa/verify", formJSON(verify2FA)).then(function (payload) {
          state.recoveryCodes = payload.recovery_codes || [];
          state.twoFactorSetup = null;
          return loadCurrentUser();
        }).then(function () { renderAccount("security"); }).catch(function (error) { showMessage("twoFactorNotice", humanError(error), true); });
      });
    }
    if (regenerate) {
      regenerate.addEventListener("submit", function (event) {
        event.preventDefault();
        jsonRequest("POST", "/auth/2fa/recovery-codes/regenerate", formJSON(regenerate)).then(function (payload) {
          state.recoveryCodes = payload.recovery_codes || [];
          renderAccount("security");
        }).catch(function (error) { showMessage("twoFactorNotice", humanError(error), true); });
      });
    }
    if (disable2FA) {
      disable2FA.addEventListener("submit", function (event) {
        event.preventDefault();
        jsonRequest("POST", "/auth/2fa/disable", formJSON(disable2FA)).then(function () {
          state.recoveryCodes = [];
          state.twoFactorSetup = null;
          return loadCurrentUser();
        }).then(function () { renderAccount("security"); }).catch(function (error) { showMessage("twoFactorNotice", humanError(error), true); });
      });
    }
    if (changePassword) {
      changePassword.addEventListener("submit", function (event) {
        event.preventDefault();
        jsonRequest("POST", "/me/password", formJSON(changePassword)).then(function () {
          changePassword.reset();
          showMessage("passwordNotice", "Password changed. Other sessions were revoked.", false);
        }).catch(function (error) { showMessage("passwordNotice", humanError(error), true); });
      });
    }
    if (changeEmail) {
      changeEmail.addEventListener("submit", function (event) {
        event.preventDefault();
        jsonRequest("POST", "/me/email", formJSON(changeEmail)).then(function (payload) {
          state.currentUser = payload.user;
          showMessage("emailNotice", "Email changed. The new address is not verified yet.", false);
          renderSidebar();
        }).catch(function (error) { showMessage("emailNotice", humanError(error), true); });
      });
    }
    if (recoveryRequest) {
      recoveryRequest.addEventListener("submit", function (event) {
        event.preventDefault();
        jsonRequest("POST", "/auth/recovery/request", formJSON(recoveryRequest)).then(function (payload) {
          showMessage("recoveryNotice", "If the account exists, a recovery link has been sent.", false);
          if (payload.recovery_token) {
            var link = "#recovery?token=" + encodeURIComponent(payload.recovery_token);
            var node = document.getElementById("recoveryNotice");
            node.innerHTML = 'Development recovery token returned. <a href="' + link + '">Open reset form</a>.';
          }
        }).catch(function (error) { showMessage("recoveryNotice", humanError(error), true); });
      });
    }
    if (recoveryReset) {
      recoveryReset.addEventListener("submit", function (event) {
        event.preventDefault();
        jsonRequest("POST", "/auth/recovery/reset", formJSON(recoveryReset)).then(function () {
          state.currentUser = null;
          state.currentSession = null;
          routeTo("login");
        }).catch(function (error) { showMessage("recoveryNotice", humanError(error), true); });
      });
    }
    if (legacyPassword) {
      legacyPassword.addEventListener("submit", function (event) {
        event.preventDefault();
        jsonRequest("POST", "/me/legacy-passwords", formJSON(legacyPassword)).then(function (payload) {
          var notice = document.getElementById("legacyTokenNotice");
          notice.hidden = false;
          notice.innerHTML = '<strong>Copy this password now:</strong><div class="mono">' + escapeHTML(payload.token) + "</div>";
          loadLegacyData();
        }).catch(function (error) {
          var notice = document.getElementById("legacyTokenNotice");
          notice.hidden = false;
          notice.textContent = humanError(error);
        });
      });
    }
  }

  function loadSecurityData() {
    var target = document.getElementById("sessionsList");
    if (!target) { return; }
    api("/me/sessions").then(function (payload) {
      var sessions = payload.sessions || [];
      target.innerHTML = sessions.length ? '<ul class="setting-list">' + sessions.map(function (session) {
        return '<li><span>' + escapeHTML(session.device_name || "Browser") + '<br><small>' + escapeHTML(session.last_seen_at || session.created_at || "") +
          '</small></span><button class="metal-button small-action" data-session-id="' + escapeHTML(session.id) + '" type="button">Revoke</button></li>';
      }).join("") + "</ul>" : '<div class="empty-state compact">No active sessions</div>';
      target.querySelectorAll("[data-session-id]").forEach(function (button) {
        button.addEventListener("click", function () {
          api("/me/sessions/" + encodeURIComponent(button.getAttribute("data-session-id")), { method: "DELETE" }).then(loadSecurityData).catch(function () {
            target.innerHTML = '<div class="empty-state compact">Unable to revoke session</div>';
          });
        });
      });
    }).catch(function () { target.innerHTML = '<div class="empty-state compact">Unable to load sessions</div>'; });
  }

  function loadLegacyData() {
    var passwordTarget = document.getElementById("legacyPasswordsList");
    var deviceTarget = document.getElementById("legacyDevicesList");
    if (passwordTarget) {
      api("/me/legacy-passwords").then(function (payload) {
        var items = payload.legacy_passwords || [];
        passwordTarget.innerHTML = items.length ? '<ul class="setting-list">' + items.map(function (item) {
          return '<li><span>' + escapeHTML(item.name) + '<br><small>' + escapeHTML(item.prefix) + " · " + escapeHTML((item.scopes || []).join(", ")) + '</small></span><span>' +
            '<button class="metal-button small-action" data-reset-token="' + escapeHTML(item.id) + '" type="button">Reset</button> ' +
            '<button class="metal-button small-action" data-delete-token="' + escapeHTML(item.id) + '" type="button">Revoke</button></span></li>';
        }).join("") + "</ul>" : '<div class="empty-state compact">No legacy passwords</div>';
        passwordTarget.querySelectorAll("[data-reset-token]").forEach(function (button) {
          button.addEventListener("click", function () {
            jsonRequest("POST", "/me/legacy-passwords/" + encodeURIComponent(button.getAttribute("data-reset-token")) + "/reset", {}).then(function (payload) {
              var notice = document.getElementById("legacyTokenNotice");
              notice.hidden = false;
              notice.innerHTML = '<strong>Copy this password now:</strong><div class="mono">' + escapeHTML(payload.token) + "</div>";
              loadLegacyData();
            });
          });
        });
        passwordTarget.querySelectorAll("[data-delete-token]").forEach(function (button) {
          button.addEventListener("click", function () {
            api("/me/legacy-passwords/" + encodeURIComponent(button.getAttribute("data-delete-token")), { method: "DELETE" }).then(loadLegacyData);
          });
        });
      }).catch(function () { passwordTarget.innerHTML = '<div class="empty-state compact">Unable to load legacy passwords</div>'; });
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
      }).catch(function () { deviceTarget.innerHTML = '<div class="empty-state compact">Unable to load devices</div>'; });
    }
  }

  function statusChip(value) {
    return '<span class="status-chip ' + escapeHTML(value || "") + '">' + escapeHTML(value || "") + "</span>";
  }

  function renderUploads() {
    if (!canUpload(state.currentUser)) { routeTo("profile"); return; }
    main.innerHTML = '<div class="view-title"><h1>Uploads</h1></div><div class="management-stack">' +
      '<section class="account-panel"><h2>1. Submit Application</h2><p class="panel-help">Trusted submissions remain pending until moderation. Moderators and administrators may publish immediately according to backend policy.</p>' +
      '<form id="uploadAppForm" class="management-form"><div class="form-grid"><div class="form-row"><label>Slug</label><input name="slug" required></div><div class="form-row"><label>Name</label><input name="name" required></div>' +
      '<div class="form-row"><label>Bundle ID</label><input name="bundle_id"></div><div class="form-row"><label>Developer</label><input name="developer_name" required></div>' +
      '<div class="form-row"><label>Category</label><select name="category_slug">' + state.categories.map(function (category) { return '<option value="' + escapeHTML(category.slug) + '">' + escapeHTML(category.name) + "</option>"; }).join("") + '</select></div>' +
      '<div class="form-row"><label>Summary</label><input name="summary" required></div><div class="form-row span-2"><label>Description</label><textarea name="description"></textarea></div></div>' +
      '<button class="blue-button" type="submit">Create Submission</button><div id="uploadAppNotice" class="form-message"></div></form></section>' +
      '<section class="account-panel"><h2>2. Create Version</h2><form id="uploadVersionForm" class="management-form"><div class="form-grid"><div class="form-row"><label>App ID</label><input name="app_id" value="' + escapeHTML(state.uploadContext.appId) + '" required></div>' +
      '<div class="form-row"><label>Version</label><input name="version" required></div><div class="form-row"><label>Release date</label><input name="release_date" type="date"></div>' +
      '<div class="form-row"><label><input name="is_recommended" type="checkbox" value="true"> Recommended</label></div><div class="form-row span-2"><label>Changelog</label><textarea name="changelog"></textarea></div></div>' +
      '<button class="blue-button" type="submit">Create Version</button><div id="uploadVersionNotice" class="form-message"></div></form></section>' +
      '<section class="account-panel"><h2>3. Upload Artifact</h2><p class="panel-help">The file is streamed into quarantine. SHA-256 is calculated server-side. Publication still requires moderation.</p>' +
      '<form id="artifactUploadForm" class="management-form"><div class="form-grid"><div class="form-row"><label>Version ID</label><input name="version_id" value="' + escapeHTML(state.uploadContext.versionId) + '" required></div>' +
      '<div class="form-row"><label>File</label><input name="file" type="file" required></div><div class="form-row"><label>Minimum macOS</label><input name="min_os" value="10.4" required></div>' +
      '<div class="form-row"><label>Maximum supported</label><input name="max_supported_os"></div><div class="form-row"><label>Maximum tested</label><input name="max_tested_os" value="10.15"></div>' +
      '<div class="form-row"><label>Install notes</label><input name="install_notes"></div></div><div class="filter-bar">' +
      '<label><input name="arch_i386" type="checkbox" value="true" checked> i386</label><label><input name="arch_x86_64" type="checkbox" value="true" checked> x86_64</label>' +
      '<label><input name="supports_32bit" type="checkbox" value="true" checked> 32-bit</label><label><input name="supports_64bit" type="checkbox" value="true" checked> 64-bit</label>' +
      '<label><input name="hard_block_above_max" type="checkbox" value="true"> Hard max OS</label></div><button class="blue-button" type="submit">Upload to Quarantine</button>' +
      '<div class="upload-progress" id="uploadProgress" hidden><span></span></div><div id="artifactUploadNotice" class="form-message"></div></form></section></div>';
    bindUploadForms();
    setStatus("Showing: Management", "Uploads", "HTTPS required");
    renderSidebar();
  }

  function bindUploadForms() {
    var appForm = document.getElementById("uploadAppForm");
    var versionForm = document.getElementById("uploadVersionForm");
    var artifactForm = document.getElementById("artifactUploadForm");
    if (appForm) {
      appForm.addEventListener("submit", function (event) {
        event.preventDefault();
        jsonRequest("POST", "/admin/apps", formJSON(appForm)).then(function (payload) {
          state.uploadContext.appId = String(payload.app.id);
          showMessage("uploadAppNotice", "Submission created as " + payload.app.moderation_status + ". App ID: " + payload.app.id, false);
          document.querySelector('#uploadVersionForm [name="app_id"]').value = payload.app.id;
        }).catch(function (error) { showMessage("uploadAppNotice", humanError(error), true); });
      });
    }
    if (versionForm) {
      versionForm.addEventListener("submit", function (event) {
        event.preventDefault();
        var data = formJSON(versionForm);
        var appId = data.app_id;
        delete data.app_id;
        data.is_recommended = !!versionForm.querySelector('[name="is_recommended"]:checked');
        jsonRequest("POST", "/admin/apps/" + encodeURIComponent(appId) + "/versions", data).then(function (payload) {
          state.uploadContext.versionId = String(payload.version.id);
          showMessage("uploadVersionNotice", "Version created. Version ID: " + payload.version.id, false);
          document.querySelector('#artifactUploadForm [name="version_id"]').value = payload.version.id;
        }).catch(function (error) { showMessage("uploadVersionNotice", humanError(error), true); });
      });
    }
    if (artifactForm) {
      artifactForm.addEventListener("submit", function (event) {
        event.preventDefault();
        var versionId = artifactForm.querySelector('[name="version_id"]').value;
        var formData = new FormData();
        ["file", "min_os", "max_supported_os", "max_tested_os", "install_notes"].forEach(function (name) {
          var input = artifactForm.querySelector('[name="' + name + '"]');
          if (input && input.type === "file") {
            if (input.files && input.files[0]) { formData.append(name, input.files[0]); }
          } else if (input) {
            formData.append(name, input.value || "");
          }
        });
        ["arch_i386", "arch_x86_64", "supports_32bit", "supports_64bit", "hard_block_above_max"].forEach(function (name) {
          var input = artifactForm.querySelector('[name="' + name + '"]');
          formData.append(name, input && input.checked ? "true" : "false");
        });
        var progress = document.getElementById("uploadProgress");
        progress.hidden = false;
        progress.querySelector("span").style.width = "35%";
        api("/admin/versions/" + encodeURIComponent(versionId) + "/upload", { method: "POST", body: formData }).then(function (payload) {
          progress.querySelector("span").style.width = "100%";
          showMessage("artifactUploadNotice", "Uploaded to quarantine. SHA-256: " + payload.upload.sha256 + ". Artifact ID: " + payload.artifact.id, false);
        }).catch(function (error) {
          progress.hidden = true;
          showMessage("artifactUploadNotice", humanError(error), true);
        });
      });
    }
  }

  function renderManagement(route) {
    if (route === "uploads") { renderUploads(); return; }
    var labels = { moderation: "Moderation Queue", "admin-dashboard": "Dashboard", users: "Users", "admin-apps": "Apps", "audit-log": "Audit Log" };
    main.innerHTML = '<div class="view-title"><h1>' + escapeHTML(labels[route] || "Management") + '</h1></div><section class="account-panel"><div id="adminContent" class="loading">Loading...</div></section>';
    setStatus("Showing: Management", labels[route] || route, "Access restricted by role");
    renderSidebar();
    loadManagement(route);
  }

  function loadManagement(route) {
    var target = document.getElementById("adminContent");
    if (!target) { return; }
    if (route === "admin-dashboard") {
      api("/admin/dashboard").then(function (payload) {
        target.innerHTML = '<ul class="setting-list">' + Object.keys(payload.dashboard || {}).sort().map(function (key) {
          return '<li><span>' + escapeHTML(key) + '</span><span>' + escapeHTML(payload.dashboard[key]) + "</span></li>";
        }).join("") + "</ul>";
      }).catch(function (error) { target.innerHTML = errorBox(error); });
      return;
    }
    if (route === "users") {
      api("/admin/users?limit=100").then(function (payload) {
        target.innerHTML = '<table class="management-table"><thead><tr><th>Email</th><th>Roles</th><th>Status</th></tr></thead><tbody>' + (payload.users || []).map(function (user) {
          return '<tr><td>' + escapeHTML(user.email) + '</td><td>' + rolesOf(user).map(function (role) { return '<span class="role-chip">' + escapeHTML(role) + "</span>"; }).join("") +
            '</td><td>' + statusChip(user.status) + "</td></tr>";
        }).join("") + "</tbody></table>";
      }).catch(function (error) { target.innerHTML = errorBox(error); });
      return;
    }
    if (route === "admin-apps") {
      api("/admin/apps?limit=100").then(function (payload) {
        target.innerHTML = '<table class="management-table"><thead><tr><th>ID</th><th>Application</th><th>Category</th><th>Status</th></tr></thead><tbody>' + (payload.apps || []).map(function (app) {
          return '<tr><td>' + escapeHTML(app.id) + '</td><td><strong>' + escapeHTML(app.name) + '</strong><br><small>' + escapeHTML(app.slug) + '</small></td><td>' +
            escapeHTML(app.category || "") + '</td><td>' + statusChip(app.moderation_status) + "</td></tr>";
        }).join("") + "</tbody></table>";
      }).catch(function (error) { target.innerHTML = errorBox(error); });
      return;
    }
    if (route === "moderation") {
      api("/admin/moderation?status=pending").then(function (payload) {
        var items = payload.items || [];
        target.innerHTML = items.length ? '<table class="management-table"><thead><tr><th>Item</th><th>Submitted</th><th></th></tr></thead><tbody>' + items.map(function (item) {
          return '<tr><td>' + escapeHTML(item.entity_type) + " #" + escapeHTML(item.entity_id) + '</td><td>' + escapeHTML(item.created_at || "") + '</td><td class="actions">' +
            '<button class="metal-button small-action" data-moderate="approve" data-id="' + escapeHTML(item.id) + '" type="button">Approve</button> ' +
            '<button class="metal-button small-action" data-moderate="reject" data-id="' + escapeHTML(item.id) + '" type="button">Reject</button></td></tr>';
        }).join("") + "</tbody></table>" : '<div class="empty-state compact">No pending moderation items</div>';
        target.querySelectorAll("[data-moderate]").forEach(function (button) {
          button.addEventListener("click", function () {
            jsonRequest("POST", "/admin/moderation/" + encodeURIComponent(button.getAttribute("data-id")) + "/" + button.getAttribute("data-moderate"), {}).then(function () { loadManagement("moderation"); });
          });
        });
      }).catch(function (error) { target.innerHTML = errorBox(error); });
      return;
    }
    if (route === "audit-log") {
      api("/admin/audit-log").then(function (payload) {
        target.innerHTML = '<table class="management-table"><thead><tr><th>Time</th><th>Action</th><th>Entity</th></tr></thead><tbody>' + (payload.items || []).map(function (item) {
          return '<tr><td>' + escapeHTML(item.created_at || "") + '</td><td>' + escapeHTML(item.action) + '</td><td>' + escapeHTML(item.entity_type || "") + " " + escapeHTML(item.entity_id || "") + "</td></tr>";
        }).join("") + "</tbody></table>";
      }).catch(function (error) { target.innerHTML = errorBox(error); });
    }
  }

  function errorBox(error) {
    return '<div class="empty-state compact">' + escapeHTML(humanError(error)) + "</div>";
  }

  function performLogout() {
    jsonRequest("POST", "/auth/logout", {}).catch(function () { return null; }).then(function () {
      state.currentUser = null;
      state.currentSession = null;
      state.twoFactorSetup = null;
      state.recoveryCodes = [];
      window.location.hash = "home";
      renderSidebar();
      render();
    });
  }

  function loadCurrentUser() {
    return api("/me").then(function (payload) {
      state.currentUser = payload.user || null;
      state.currentSession = payload.session || null;
      return state.currentUser;
    }).catch(function () {
      state.currentUser = null;
      state.currentSession = null;
      return null;
    });
  }

  function renderError(error) {
    main.innerHTML = '<div class="empty-state">Error: ' + escapeHTML(humanError(error)) + "</div>";
  }

  function render() {
    var parsed = parseRoute();
    state.route = parsed.route;
    state.routeParams = parsed.params;
    main.classList.remove("is-auth");
    setActiveTabs();

    if (state.route === "home") { renderHome(); return; }
    if (state.route === "catalog") { renderCatalog(""); return; }
    if (state.route.indexOf("category/") === 0) { renderCatalog(state.route.split("/")[1]); return; }
    if (state.route === "categories") { renderCategories(); return; }
    if (state.route === "search") { renderSearch(parsed.params.get("q") || ""); return; }
    if (state.route.indexOf("app/") === 0) { renderApp(state.route.split("/")[1]); return; }
    if (state.route === "top-charts") { renderTopCharts(); return; }
    if (state.route === "logout") { performLogout(); return; }

    if (["profile", "security", "legacy-passwords"].indexOf(state.route) >= 0 && !state.currentUser) {
      routeTo("login"); return;
    }
    if (["login", "register", "recovery", "profile", "security", "legacy-passwords"].indexOf(state.route) >= 0) {
      renderAccount(state.route, parsed.params); return;
    }
    if (["uploads", "moderation", "admin-dashboard", "users", "admin-apps", "audit-log"].indexOf(state.route) >= 0) {
      var allowed = state.route === "uploads" ? canUpload(state.currentUser) : (state.route === "users" || state.route === "audit-log" ? isAdmin(state.currentUser) : canModerate(state.currentUser));
      if (!allowed) { routeTo(state.currentUser ? "profile" : "login"); return; }
      renderManagement(state.route); return;
    }
    routeTo("home");
  }

  document.querySelectorAll(".toolbar-tab").forEach(function (button) {
    button.addEventListener("click", function () { routeTo(button.getAttribute("data-route")); });
  });
  document.getElementById("backButton").addEventListener("click", function () { history.back(); });
  document.getElementById("forwardButton").addEventListener("click", function () { history.forward(); });
  searchForm.addEventListener("submit", function (event) {
    event.preventDefault();
    var query = searchInput.value.trim();
    if (query) { routeTo("search?q=" + encodeURIComponent(query)); }
  });
  window.addEventListener("hashchange", render);

  Promise.all([
    api("/categories").then(function (payload) { state.categories = normalizeAppleCategories(payload.categories || []); }),
    loadCurrentUser()
  ]).then(function () {
    renderSidebar();
    render();
  }).catch(renderError);
})();
