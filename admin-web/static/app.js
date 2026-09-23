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
    carouselTimer: null,
    uploadContext: { appId: "", versionId: "", stageUid: "", file: null, inspection: null, iconDataURL: "" }
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
    return hasRole(user, "uploader", "trusted", "moder", "admin");
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
      managementRoutes.push(["uploads", "Add Application", "upload"]);
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

  function loadApps(category, limit, sortMode) {
    var path = "/apps?" + queryTarget() + "&limit=" + encodeURIComponent(limit || 24) + "&page=1";
    if (category) {
      path += "&category=" + encodeURIComponent(category);
    }
    if (sortMode) {
      path += "&sort=" + encodeURIComponent(sortMode);
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

  function finderMarkHTML() {
    return '<div class="finder-mark" aria-hidden="true"><svg viewBox="0 0 180 180">' +
      '<defs><linearGradient id="finderLeft" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#7dc2ff"/><stop offset="1" stop-color="#1870cf"/></linearGradient>' +
      '<linearGradient id="finderRight" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#eaf6ff"/><stop offset="1" stop-color="#92caf5"/></linearGradient></defs>' +
      '<rect x="8" y="8" width="164" height="164" rx="24" fill="url(#finderRight)"/><path d="M8 8h80l-18 66 22 98H32c-13 0-24-11-24-24V8Z" fill="url(#finderLeft)"/>' +
      '<path d="M88 8v164" stroke="#275d9a" stroke-width="4"/><path d="M47 67c8-7 17-7 25 0M110 67c8-7 17-7 25 0" fill="none" stroke="#173a64" stroke-width="6" stroke-linecap="round"/>' +
      '<path d="M58 118c20 17 45 20 68 1" fill="none" stroke="#173a64" stroke-width="6" stroke-linecap="round"/></svg></div>';
  }

  function welcomeSlideHTML() {
    return '<article class="home-carousel-slide is-welcome" data-carousel-slide>' +
      '<img class="home-carousel-art" data-home-carousel-img src="/assets/catalog-hero.png" alt="">' +
      '<div class="home-carousel-shade"></div><div class="home-carousel-content"><span class="carousel-eyebrow">LegacyStore</span>' +
      '<h1>Welcome to Legacy Store!</h1><p>Classic software catalog for old Intel Macs.</p>' +
      '<button class="blue-button" data-route="catalog" type="button">Browse Applications</button></div></article>';
  }

  function emptyCatalogSlideHTML() {
    return '<article class="home-carousel-slide is-empty" data-carousel-slide>' + finderMarkHTML() +
      '<div class="home-carousel-content"><span class="carousel-eyebrow">Fresh installation</span>' +
      '<h1>No applications yet</h1><p>The apps haven\'t been uploaded yet. Maybe you\'re a developer and just started this Legacy Store instance, huh? ;-)</p>' +
      '<button class="blue-button" data-route="uploads" type="button">Upload Application</button></div></article>';
  }

  function appCarouselSlideHTML(slide) {
    var app = (slide && slide.app) || {};
    var image = app.hero_image || app.icon || "";
    var imageClass = app.hero_image ? " hero-shot" : " app-icon-art";
    return '<article class="home-carousel-slide is-app" data-carousel-slide>' +
      (image ? '<img class="home-carousel-art' + imageClass + '" data-home-carousel-img src="' + escapeHTML(image) + '" alt="">' : "") +
      '<div class="home-carousel-shade"></div><div class="home-carousel-content"><span class="carousel-eyebrow">' +
      escapeHTML(slide.metric || "Featured") + '</span><h1>' + escapeHTML(app.name || "Application") + '</h1><p>' +
      escapeHTML(app.summary || "") + '</p><button class="blue-button" data-route="app/' + escapeHTML(app.slug || "") +
      '" type="button">View Application</button></div></article>';
  }

  function stopHomeCarousel() {
    if (state.carouselTimer) {
      window.clearInterval(state.carouselTimer);
      state.carouselTimer = null;
    }
  }

  function bindHomeCarousel() {
    stopHomeCarousel();
    var carousel = document.getElementById("homeCarousel");
    if (!carousel) { return; }
    var track = carousel.querySelector("[data-carousel-track]");
    var slides = Array.prototype.slice.call(carousel.querySelectorAll("[data-carousel-slide]"));
    var dots = Array.prototype.slice.call(carousel.querySelectorAll("[data-carousel-index]"));
    if (!track || !slides.length) { return; }

    var current = 0;
    function activate(index) {
      current = (index + slides.length) % slides.length;
      track.style.transform = "translateX(-" + (current * 100) + "%)";
      slides.forEach(function (slide, position) {
        slide.classList.toggle("is-active", position === current);
        slide.setAttribute("aria-hidden", position === current ? "false" : "true");
      });
      dots.forEach(function (dot, position) {
        dot.classList.toggle("is-active", position === current);
        dot.setAttribute("aria-current", position === current ? "true" : "false");
      });
    }

    dots.forEach(function (dot) {
      dot.addEventListener("click", function () {
        activate(Number(dot.getAttribute("data-carousel-index") || 0));
      });
    });

    function start() {
      stopHomeCarousel();
      if (slides.length > 1) {
        state.carouselTimer = window.setInterval(function () { activate(current + 1); }, 6000);
      }
    }
    carousel.addEventListener("mouseenter", stopHomeCarousel);
    carousel.addEventListener("mouseleave", start);
    carousel.addEventListener("focusin", stopHomeCarousel);
    carousel.addEventListener("focusout", start);

    carousel.querySelectorAll("[data-home-carousel-img]").forEach(function (image) {
      image.addEventListener("error", function () { image.classList.add("is-broken"); }, { once: true });
      if (image.complete && image.naturalWidth === 0) { image.classList.add("is-broken"); }
    });

    activate(0);
    start();
  }

  function homeCarouselHTML(serverSlides) {
    var slides = [welcomeSlideHTML()];
    if (serverSlides && serverSlides.length) {
      serverSlides.forEach(function (slide) { slides.push(appCarouselSlideHTML(slide)); });
    } else {
      slides.push(emptyCatalogSlideHTML());
    }
    var dots = slides.map(function (_, index) {
      return '<button class="home-carousel-dot' + (index === 0 ? " is-active" : "") +
        '" data-carousel-index="' + index + '" type="button" aria-label="Show slide ' + (index + 1) + '"></button>';
    }).join("");
    return '<section class="home-carousel" id="homeCarousel"><div class="home-carousel-track" data-carousel-track>' +
      slides.join("") + '</div><div class="home-carousel-dots">' + dots + '</div></section>';
  }

  function renderHome() {
    state.category = "";
    main.innerHTML = '<div class="loading">Loading...</div>';
    api("/home?" + queryTarget()).then(function (feed) {
      state.apps = feed.popular || [];
      main.innerHTML = homeCarouselHTML(feed.slides || []) +
        panel("Popular", "top-charts", (feed.popular || []).map(appCard).join("")) +
        panel("Top Downloads", "catalog?sort=downloads", (feed.top_downloads || []).map(appCard).join("")) +
        panel("New Releases", "catalog?sort=new", (feed.new_releases || []).map(appCard).join(""));
      bindRouteButtons(main);
      bindImageFallbacks(main);
      bindHomeCarousel();
      setStatus("Showing: Featured", "", "Server-ranked selections");
      renderSidebar();
    }).catch(renderError);
  }

  function renderCatalog(category, sortMode) {
    state.category = category || "";
    main.innerHTML = '<div class="loading">Loading...</div>';
    loadApps(state.category, 50, sortMode || "").then(function (apps) {
      state.apps = apps;
      var title = categoryName(state.category) || (sortMode === "downloads" ? "Top Downloads" : (sortMode === "new" ? "New Releases" : "All Software"));
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
          escapeHTML((artifact.architecture_labels || artifact.archs || []).join(", ")) + '</td><td>' + escapeHTML(artifactSupportedSystemsLabel(artifact)) + '</td><td>' +
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
        escapeHTML(app.developer_name || "") + '</div><p>' + escapeHTML(app.summary || "") + '</p><div class="badge-row">' + badges(artifact.architecture_labels || artifact.archs || []) +
        compatibilityPill(app.compatibility) + '</div></div><div class="detail-actions"><button class="blue-button" id="downloadButton" type="button"' +
        (!artifact.id || blocked ? " disabled" : "") + '>Download</button>' +
        (canUpload(state.currentUser) && app.id ? '<button class="metal-button" data-route="upload-version/' + escapeHTML(app.id) + '" type="button">Upload Version</button>' : '') +
        '<button class="metal-button" data-route="catalog" type="button">Back to Catalog</button></div></section>' +
        '<div class="detail-grid"><section class="detail-copy"><h2>Description</h2><p>' + escapeHTML(app.description || "") + '</p>' + screenshotStrip(app) +
        compatibilityBlock(app.compatibility) + versionsTable(app.versions || [], app) + reviewsHTML(app) + '</section><aside class="side-box"><h2>Information</h2>' +
        infoRow("Category", app.category) + infoRow("Version", artifact.version) + infoRow("Size", formatSize(artifact.size_bytes)) + infoRow("Package", artifact.package_type) +
        infoRow("Downloads", app.downloads) + infoRow("Unique views", app.views) +
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
      jsonRequest("POST", "/apps/" + encodeURIComponent(app.slug) + "/view", {}).catch(function () {});
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

  function reviewsHTML(app) {
    var versions = (app.versions || []).map(function (version) { return version.version; });
    var versionOptions = '<option value="">Unknown / not installed</option>' + versions.map(function (version) {
      return '<option value="' + escapeHTML(version) + '">' + escapeHTML(version) + '</option>';
    }).join("");
    var form = state.currentUser ? '<form id="reviewForm" class="review-form" data-slug="' + escapeHTML(app.slug) + '"><h2>Your Review</h2>' +
      '<div class="compact-field-grid"><div class="form-row field-version"><label>Rating</label><select name="rating" aria-label="Rating">' +
      '<option>5</option><option>4</option><option>3</option><option>2</option><option>1</option></select></div>' +
      '<div class="form-row field-medium"><label>Application version</label><select name="app_version">' + versionOptions + '</select>' +
      '<small>Select the version you actually used.</small></div>' +
      '<div class="form-row field-wide"><label>Title</label><input name="title" type="text" maxlength="120" placeholder="Short summary"></div></div>' +
      '<div class="form-row"><label>Review</label><textarea name="body" maxlength="300" placeholder="Up to 300 characters" required></textarea>' +
      '<small>Maximum 300 characters. Browser/OS information is recorded only when available.</small></div>' +
      '<div class="form-row"><label>Images (optional)</label><input name="images" type="file" accept="image/jpeg,image/png,image/gif" multiple>' +
      '<small>Up to 3 images, 2 MB each, maximum 2048×2048. Images are recompressed and stored at no more than 1 MB each.</small></div>' +
      '<button class="blue-button" type="submit">Post Review</button><div id="reviewNotice" class="form-message"></div></form>' : "";
    return '<section class="reviews-section">' + form + '<h2>Reviews</h2><div id="reviewsList" class="loading">Loading...</div></section>';
  }

  function reviewContextLine(review) {
    var parts = [];
    if (review.app_version) { parts.push("App " + review.app_version); }
    if (review.os_version) { parts.push("OS X " + review.os_version); }
    if (review.os_arch) { parts.push(review.os_arch); }
    if (review.device_model) { parts.push(review.device_model); }
    if (review.client_version) { parts.push("LegacyStore " + review.client_version); }
    return parts.join(" · ");
  }

  function loadReviews(slug) {
    var target = document.getElementById("reviewsList");
    if (!target) { return; }
    var form = document.getElementById("reviewForm");
    if (form) {
      form.addEventListener("submit", function (event) {
        event.preventDefault();
        var data = formJSON(form);
        delete data.images;
        data.rating = Number(data.rating || 5);
        var imageInput = form.querySelector('[name="images"]');
        var files = imageInput && imageInput.files ? Array.prototype.slice.call(imageInput.files) : [];
        if (files.length > 3) {
          showMessage("reviewNotice", "A review can contain at most 3 images.", true);
          return;
        }
        if (files.some(function (file) { return file.size > 2 * 1024 * 1024; })) {
          showMessage("reviewNotice", "Each review image must be 2 MB or smaller.", true);
          return;
        }
        jsonRequest("POST", "/apps/" + encodeURIComponent(slug) + "/reviews", data).then(function (payload) {
          if (!files.length) { return payload; }
          var formData = new FormData();
          files.forEach(function (file) { formData.append("images", file, file.name); });
          return api("/reviews/" + encodeURIComponent(payload.review.uid) + "/images", { method: "POST", body: formData });
        }).then(function () {
          form.reset();
          return loadReviews(slug);
        }).catch(function (error) { showMessage("reviewNotice", humanError(error), true); });
      });
    }
    api("/apps/" + encodeURIComponent(slug) + "/reviews").then(function (payload) {
      var reviews = payload.reviews || [];
      target.innerHTML = reviews.length ? '<ul class="setting-list review-list">' + reviews.map(function (review) {
        var context = reviewContextLine(review);
        var avatar = review.avatar_url ?
          '<img class="review-avatar" src="' + escapeHTML(review.avatar_url) + '" alt="">' :
          '<span class="review-avatar review-avatar-fallback">' + escapeHTML((review.author || "U").slice(0, 1).toUpperCase()) + '</span>';
        var images = (review.images || []).length ? '<div class="review-image-strip">' + (review.images || []).map(function (image) {
          return '<a href="' + escapeHTML(image.url) + '" target="_blank" rel="noopener"><img class="review-upload-image" src="' +
            escapeHTML(image.url) + '" alt="Review image" loading="lazy"></a>';
        }).join("") + '</div>' : "";
        return '<li><span class="review-main">' + avatar + '<span><strong>' + escapeHTML(review.author || "User") + '</strong> ' + ratingDots(review.rating) +
          (context ? '<br><small class="review-context">' + escapeHTML(context) + '</small>' : "") +
          '<br>' + escapeHTML(review.title || "") + '<br><small>' + escapeHTML(review.body || "") + '</small>' + images + '</span></span><span>' +
          escapeHTML(review.likes || 0) + ' likes' +
          (state.currentUser ? ' <button class="metal-button small-action" data-like-review="' + escapeHTML(review.uid || "") + '" type="button">Like</button>' : "") +
          '</span></li>';
      }).join("") + "</ul>" : '<div class="empty-state compact">No reviews</div>';
      target.querySelectorAll("[data-like-review]").forEach(function (button) {
        button.addEventListener("click", function () {
          jsonRequest("POST", "/reviews/" + encodeURIComponent(button.getAttribute("data-like-review")) + "/like", {}).then(function () { loadReviews(slug); });
        });
      });
      bindImageFallbacks(target);
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
    loadApps("", 50, "popular").then(function (apps) {
      main.innerHTML = '<div class="view-title"><h1>Top Charts</h1>' + filtersHTML() + '</div><div class="app-list">' + apps.map(appRow).join("") + "</div>";
      bindRouteButtons(main); bindFilters(); bindImageFallbacks(main);
      setStatus("Showing: Top Charts", apps.length + " items", "Sort By: Popularity");
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
      '<form id="twoFactorLoginForm"><div class="second-factor-inline"><div class="form-row field-totp"><label>Authenticator code</label>' +
      '<input name="totp_code" inputmode="numeric" autocomplete="one-time-code" maxlength="6" pattern="[0-9]{6}" placeholder="123456"><small>6 digits.</small></div>' +
      '<span class="or">or</span><div class="form-row field-recovery"><label>Recovery code</label><input name="recovery_code" autocomplete="one-time-code" placeholder="XXXX-XXXX-XXXX"></div></div>' +
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
    var initials = String(user.nickname || user.email || "LS").split(/\s+/).map(function (part) { return part.charAt(0); }).join("").slice(0, 2).toUpperCase();
    var avatar = user.avatar_url ?
      '<img class="profile-avatar-image" src="' + escapeHTML(user.avatar_url) + '" alt="">' :
      '<span class="profile-avatar-fallback">' + escapeHTML(initials) + '</span>';
    return '<div class="profile-bento">' +
      '<section class="bento-card profile-identity-card"><div class="profile-avatar">' + avatar + '</div><div class="profile-identity-copy"><h2>' +
      escapeHTML(user.nickname || "LegacyStore User") + '</h2><p>' + escapeHTML(user.email || "") + '</p><div class="profile-role-row">' +
      rolesOf(user).map(function (role) { return '<span class="role-chip">' + escapeHTML(role) + '</span>'; }).join("") + '</div></div></section>' +
      '<section class="bento-card profile-edit-card"><h2>Profile details</h2><p class="panel-help">Public nickname and optional avatar used on reviews and account pages.</p>' +
      '<form id="profileForm"><div class="compact-field-grid"><div class="form-row field-medium"><label>Nickname</label><input name="nickname" type="text" maxlength="80" ' +
      'placeholder="Steve Jobs" value="' + escapeHTML(user.nickname || "") + '"><small>Shown next to your reviews and submissions.</small></div>' +
      '<div class="form-row field-wide"><label>Avatar image</label><input name="avatar_file" type="file" accept="image/jpeg,image/png,image/gif">' +
      '<small>JPEG/PNG/GIF, up to 2 MB and 2048×2048. Stored at up to 512×512 and compressed to at most 1 MB.</small></div></div>' +
      '<div id="profileNotice" class="form-message"></div><div class="form-actions"><button class="blue-button" type="submit">Save profile</button>' +
      (user.avatar_url ? '<button class="metal-button" id="deleteAvatarButton" type="button">Remove avatar</button>' : "") + '</div></form></section>' +
      '<section class="bento-card profile-status-card"><h2>Account</h2><div class="account-metric-grid">' +
      '<div class="account-metric"><span>Email</span><strong>' + (user.email_verified ? "Verified" : "Not verified") + '</strong></div>' +
      '<div class="account-metric"><span>Two-factor authentication</span><strong>' + (user.two_factor_enabled ? "Enabled" : "Off") + '</strong></div></div>' +
      '<button class="metal-button" data-route="security" type="button">Security settings</button></section></div>';
  }

  function secondFactorFields(prefix) {
    return '<div class="second-factor-inline"><div class="form-row field-totp"><label>Authenticator code</label>' +
      '<input name="totp_code" inputmode="numeric" autocomplete="one-time-code" maxlength="6" pattern="[0-9]{6}" placeholder="123456">' +
      '<small>6 digits from your authenticator.</small></div><span class="or">or</span>' +
      '<div class="form-row field-recovery"><label>Recovery code</label><input name="recovery_code" autocomplete="one-time-code" placeholder="XXXX-XXXX-XXXX">' +
      '<small>Use one recovery code instead.</small></div></div>' + (prefix ? "" : "");
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
      twoFactorBody = '<p class="panel-help">Protect the account with a six-digit TOTP code. Your current password is required before a secret is generated.</p>' +
        '<form id="setup2FAForm"><div class="compact-field-grid"><div class="form-row field-medium"><label>Current password</label>' +
        '<input name="current_password" type="password" autocomplete="current-password" placeholder="Current password" required></div></div>' +
        '<div class="form-actions"><button class="metal-button" type="submit">Generate 2FA secret</button></div></form>' +
        (state.twoFactorSetup ? '<div class="secret-box"><strong>Authenticator secret</strong><div class="mono">' + escapeHTML(state.twoFactorSetup.secret || "") +
          '</div><div class="mono secret-url">' + escapeHTML(state.twoFactorSetup.otpauth_url || "") + '</div></div>' +
          '<form id="verify2FAForm"><div class="compact-field-grid"><div class="form-row field-totp"><label>Confirm code</label>' +
          '<input name="totp_code" inputmode="numeric" autocomplete="one-time-code" maxlength="6" pattern="[0-9]{6}" placeholder="123456" required>' +
          '<small>Enter the current six-digit code.</small></div></div><div class="form-actions"><button class="blue-button" type="submit">Enable 2FA</button></div></form>' : "");
    } else {
      twoFactorBody = '<p class="panel-help">Two-factor authentication is enabled. Recovery codes are single-use.</p>' + recoveryCodesHTML() +
        '<div class="security-mini-grid"><form id="regenerateRecoveryCodesForm" class="security-mini-card"><h3>New recovery codes</h3>' +
        '<div class="form-row field-medium"><label>Current password</label><input name="current_password" type="password" autocomplete="current-password" placeholder="Current password" required></div>' +
        secondFactorFields("regenerate") + '<div class="form-actions"><button class="metal-button" type="submit">Regenerate</button></div></form>' +
        '<form id="disable2FAForm" class="security-mini-card danger-zone"><h3>Disable 2FA</h3>' +
        '<div class="form-row field-medium"><label>Current password</label><input name="current_password" type="password" autocomplete="current-password" placeholder="Current password" required></div>' +
        secondFactorFields("disable") + '<div class="form-actions"><button class="metal-button" type="submit">Disable 2FA</button></div></form></div>';
    }

    return '<div class="security-bento"><section class="bento-card security-2fa-card"><div class="card-heading-row"><div><h2>Two-Factor Authentication</h2>' +
      '<p class="card-kicker">' + (enabled ? "Enabled" : "Not enabled") + '</p></div><span class="status-chip ' + (enabled ? "active" : "") + '">' +
      (enabled ? "Enabled" : "Off") + '</span></div>' + twoFactorBody + '<div id="twoFactorNotice" class="form-message"></div></section>' +
      '<section class="bento-card security-password-card"><h2>Change Password</h2><p class="panel-help">Other active sessions are revoked after a password change.</p>' +
      '<form id="changePasswordForm"><div class="compact-field-grid"><div class="form-row field-medium"><label>Current password</label>' +
      '<input name="current_password" type="password" autocomplete="current-password" placeholder="Current password" required></div>' +
      '<div class="form-row field-medium"><label>New password</label><input name="new_password" type="password" autocomplete="new-password" minlength="8" placeholder="At least 8 characters" required></div></div>' +
      secondFactorFields("password") + '<div id="passwordNotice" class="form-message"></div><div class="form-actions"><button class="blue-button" type="submit">Change Password</button></div></form></section>' +
      '<section class="bento-card security-email-card"><h2>Change Email</h2><p class="panel-help">The new address must be verified again.</p>' +
      '<form id="changeEmailForm"><div class="compact-field-grid"><div class="form-row field-wide"><label>New email</label><input name="new_email" type="email" autocomplete="email" placeholder="me@example.com" required></div>' +
      '<div class="form-row field-medium"><label>Current password</label><input name="current_password" type="password" autocomplete="current-password" placeholder="Current password" required></div></div>' +
      secondFactorFields("email") + '<div id="emailNotice" class="form-message"></div><div class="form-actions"><button class="blue-button" type="submit">Change Email</button></div></form></section>' +
      '<section class="bento-card security-sessions-card"><h2>Active Sessions</h2><p class="panel-help">Revoke browsers or devices you no longer use.</p><div id="sessionsList" class="loading">Loading...</div></section></div>';
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
        var avatarInput = profile.querySelector('[name="avatar_file"]');
        var avatarFile = avatarInput && avatarInput.files ? avatarInput.files[0] : null;
        if (avatarFile && avatarFile.size > 2 * 1024 * 1024) {
          showMessage("profileNotice", "Avatar must be 2 MB or smaller.", true);
          return;
        }
        jsonRequest("PATCH", "/me", {
          nickname: profile.querySelector('[name="nickname"]').value || "",
          avatar_url: ""
        }).then(function (payload) {
          state.currentUser = payload.user;
          if (!avatarFile) { return payload; }
          var formData = new FormData();
          formData.append("file", avatarFile, avatarFile.name);
          return api("/me/avatar", { method: "POST", body: formData });
        }).then(function () {
          return api("/me");
        }).then(function (payload) {
          state.currentUser = payload.user;
          showMessage("profileNotice", "Saved", false);
          renderSidebar();
          renderAccount("profile");
        }).catch(function (error) { showMessage("profileNotice", humanError(error), true); });
      });
      var deleteAvatarButton = document.getElementById("deleteAvatarButton");
      if (deleteAvatarButton) {
        deleteAvatarButton.addEventListener("click", function () {
          api("/me/avatar", { method: "DELETE" }).then(function () { return api("/me"); }).then(function (payload) {
            state.currentUser = payload.user;
            renderSidebar();
            renderAccount("profile");
          }).catch(function (error) { showMessage("profileNotice", humanError(error), true); });
        });
      }
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

  function uploadCategoryOptions() {
    return '<option value="">Choose a category...</option>' + state.categories.map(function (category) {
      return '<option value="' + escapeHTML(category.slug) + '">' + escapeHTML(category.name) + '</option>';
    }).join("");
  }

  function slugifyAppName(value) {
    return String(value || "").trim().toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "")
      .slice(0, 80);
  }

  
function renderUploads() {
    if (!canUpload(state.currentUser)) { routeTo("profile"); return; }
    state.uploadContext = { appId: "", versionId: "", stageUid: "", file: null, inspection: null, iconDataURL: "" };
    main.innerHTML =
      '<div class="view-title upload-page-title"><div><h1>Create Application</h1>' +
      '<p class="view-subtitle">Create the catalog page first. Releases and binaries are uploaded separately from the application page.</p></div></div>' +
      '<form id="createApplicationForm" class="upload-bento create-app-form">' +
      '<section class="bento-card upload-app-card"><h2>Application</h2><p class="panel-help">These fields describe the application itself and are shared by every release.</p>' +
      '<div class="compact-field-grid upload-app-fields">' +
      '<div class="form-row field-large"><label>App Name <span class="required-dot">*</span></label><input name="name" type="text" maxlength="160" placeholder="My cool Macintosh app!" required></div>' +
      '<div class="form-row field-medium"><label>Developer <span class="required-dot">*</span></label><input name="developer_name" type="text" maxlength="160" placeholder="Developer or company" required></div>' +
      '<div class="form-row field-wide"><label>Bundle ID</label><input name="bundle_id" type="text" maxlength="255" placeholder="com.example.mycoolapp"></div>' +
      '<div class="form-row field-medium"><label>Category <span class="required-dot">*</span></label><select name="category_slug" required>' + uploadCategoryOptions() + '</select></div>' +
      '<div class="form-row field-medium"><label>Catalog slug <span class="required-dot">*</span></label><input name="slug" type="text" maxlength="80" placeholder="my-cool-macintosh-app" required></div>' +
      '<div class="form-row field-large"><label>Summary <span class="required-dot">*</span></label><input name="summary" type="text" maxlength="240" placeholder="A short one-line description" required></div>' +
      '<div class="form-row field-wide"><label>Homepage</label><input name="website_url" type="url" maxlength="1000" placeholder="https://example.com/"></div>' +
      '<div class="form-row field-wide"><label>Source / GitHub</label><input name="source_url" type="url" maxlength="1000" placeholder="https://github.com/example/project"></div>' +
      '<div class="form-row field-full"><label>Description</label><textarea name="description" placeholder="What does this application do?"></textarea></div>' +
      '</div></section>' +
      '<section class="bento-card upload-media-card"><h2>Default Icon</h2><p class="panel-help">Optional application-level icon. A release can later override it with its own version-specific icon.</p>' +
      '<div class="form-row field-wide"><label>Application icon</label><input name="icon_file" type="file" accept="image/jpeg,image/png,image/gif,image/svg+xml,.svg">' +
      '<small>Up to 2 MB. LegacyStore validates and recompresses the image before storage.</small></div></section>' +
      '<section class="bento-card upload-submit-card"><div><h2>Create catalog page</h2><p>The page is created first. After that you can upload one or more releases from its contributor page.</p></div>' +
      '<div class="upload-submit-actions"><button class="blue-button upload-primary-button" id="createApplicationButton" type="submit">Create Application</button></div>' +
      '<div id="createApplicationNotice" class="form-message"></div></section></form>';

    bindCreateApplicationForm();
    setStatus("Showing: Management", "Create Application", "Application metadata only");
    renderSidebar();
  }

  function bindCreateApplicationForm() {
    var form = document.getElementById("createApplicationForm");
    if (!form) { return; }
    var nameInput = form.querySelector('[name="name"]');
    var slugInput = form.querySelector('[name="slug"]');
    if (nameInput && slugInput) {
      nameInput.addEventListener("input", function () {
        if (!slugInput.value || slugInput.dataset.generated === "true") {
          slugInput.value = slugifyAppName(nameInput.value);
          slugInput.dataset.generated = "true";
        }
      });
      slugInput.addEventListener("input", function () { slugInput.dataset.generated = "false"; });
    }

    form.addEventListener("submit", function (event) {
      event.preventDefault();
      var iconInput = form.querySelector('[name="icon_file"]');
      var icon = iconInput && iconInput.files ? iconInput.files[0] : null;
      if (icon && icon.size > 2 * 1024 * 1024) {
        showMessage("createApplicationNotice", "Application icon must be 2 MB or smaller.", true);
        return;
      }
      var data = formJSON(form);
      var button = document.getElementById("createApplicationButton");
      button.disabled = true;
      showMessage("createApplicationNotice", "Creating application page...", false);
      jsonRequest("POST", "/admin/apps", {
        slug: data.slug,
        name: data.name,
        bundle_id: data.bundle_id || "",
        developer_name: data.developer_name,
        category_slug: data.category_slug,
        summary: data.summary,
        description: data.description || "",
        website_url: data.website_url || "",
        source_url: data.source_url || ""
      }).then(function (payload) {
        var app = payload.app || {};
        state.uploadContext.appId = String(app.id || "");
        if (!icon) { return payload; }
        var formData = new FormData();
        formData.append("file", icon, icon.name || "icon");
        formData.append("app_version_id", "0");
        formData.append("min_os", "10.4");
        formData.append("max_os", "15");
        return api("/admin/apps/" + encodeURIComponent(app.id) + "/icons/upload", { method: "POST", body: formData }).then(function () { return payload; });
      }).then(function (payload) {
        var status = payload.app && payload.app.moderation_status ? payload.app.moderation_status : "pending";
        if (status === "pending") {
          showToast("Application page created and sent to moderation. You can upload releases now.", "success");
        } else {
          showToast("Application page created.", "success");
        }
        routeTo("manage-app/" + encodeURIComponent(payload.app.id));
      }).catch(function (error) {
        button.disabled = false;
        showMessage("createApplicationNotice", humanError(error), true);
      });
    });
  }

  function renderContributorApp(appId) {
    if (!canUpload(state.currentUser)) { routeTo("profile"); return; }
    main.innerHTML = '<div class="loading">Loading application...</div>';
    api("/admin/apps/" + encodeURIComponent(appId)).then(function (payload) {
      var app = payload.app || {};
      var versions = payload.versions || [];
      var requests = versions.map(function (version) {
        return api("/admin/versions/" + encodeURIComponent(version.id) + "/artifacts").then(function (artifactPayload) {
          version.artifacts = artifactPayload.artifacts || [];
          return version;
        }).catch(function () {
          version.artifacts = [];
          return version;
        });
      });
      return Promise.all(requests).then(function () { return { app: app, versions: versions }; });
    }).then(function (payload) {
      var app = payload.app || {};
      var versions = payload.versions || [];
      var iconHTML = app.icon ?
        '<div class="app-icon"><span class="app-icon-initials">' + escapeHTML(appInitials(app)) + '</span><img class="app-icon-image" data-app-icon-img src="' + escapeHTML(app.icon) + '" alt=""></div>' :
        appIconHTML(app, false);

      var rows = versions.length ? versions.map(function (version) {
        var artifacts = version.artifacts || [];
        var status = "metadata only";
        if (artifacts.some(function (artifact) { return artifact.moderation_status === "approved"; })) {
          status = "approved";
        } else if (artifacts.some(function (artifact) { return artifact.moderation_status === "pending"; })) {
          status = "pending";
        } else if (artifacts.some(function (artifact) { return artifact.moderation_status === "rejected"; })) {
          status = "rejected";
        }
        var arch = [];
        artifacts.forEach(function (artifact) {
          (artifact.architectures || []).forEach(function (code) {
            if (arch.indexOf(code) < 0) { arch.push(code); }
          });
        });
        return '<tr><td><strong>' + escapeHTML(version.version) + '</strong></td><td>' + escapeHTML(version.release_date || "") + '</td><td>' +
          escapeHTML(humanArchitectureLabels(arch).join(", ") || "—") + '</td><td>' + statusChip(status) +
          (version.is_recommended ? ' <span class="status-chip active">Recommended</span>' : '') + '</td></tr>';
      }).join("") : '<tr><td colspan="4" class="muted">No releases yet.</td></tr>';

      var moderationNote = app.moderation_status === "pending" ?
        '<div class="web-security-note contribution-moderation-note">This application page is awaiting moderation. You can continue uploading releases; nothing becomes public until it is approved.</div>' :
        (app.moderation_status === "rejected" ? '<div class="web-security-note contribution-moderation-note is-error">This application page was rejected. A moderator must approve the page before it can be published.</div>' : '');

      main.innerHTML =
        '<section class="detail-head contributor-app-head">' + iconHTML +
        '<div><div class="muted">Contributor application page</div><h1>' + escapeHTML(app.name || "Application") + '</h1>' +
        '<p>' + escapeHTML(app.summary || "") + '</p><div class="badge-row">' + statusChip(app.moderation_status || "pending") + '</div></div>' +
        '<div class="detail-actions"><button class="blue-button" id="uploadVersionButton" type="button">Upload Version</button>' +
        (app.slug && app.moderation_status === "approved" ? '<button class="metal-button" data-route="app/' + escapeHTML(app.slug) + '" type="button">Public Page</button>' : '') +
        '</div></section>' + moderationNote +
        '<div class="detail-grid"><section class="detail-copy">' +
        '<section class="section-panel contributor-summary"><div class="section-header"><h2>Application metadata</h2></div>' +
        '<div class="contributor-metadata-grid">' +
        infoRow("Developer", app.developer_name) + infoRow("Bundle ID", app.bundle_id) + infoRow("Category", app.category) +
        infoRow("Homepage", app.website_url) + infoRow("Source", app.source_url) + '</div>' +
        '<p>' + escapeHTML(app.description || "") + '</p></section>' +
        '<h2>Releases</h2><table class="version-table"><thead><tr><th>Version</th><th>Date</th><th>Architecture</th><th>Status</th></tr></thead><tbody>' + rows + '</tbody></table>' +
        '</section><aside class="side-box"><h2>Contribution</h2><p class="muted">Application ID: <span class="mono">' + escapeHTML(app.id) + '</span></p>' +
        '<p>Upload binaries as separate releases. Each release gets its own compatibility metadata and optional version-specific icon.</p>' +
        '<button class="blue-button full-width" id="uploadVersionButtonAside" type="button">Upload Version</button></aside></div>';

      bindRouteButtons(main);
      bindImageFallbacks(main);
      function goUpload() { routeTo("upload-version/" + encodeURIComponent(app.id)); }
      var a = document.getElementById("uploadVersionButton");
      var b = document.getElementById("uploadVersionButtonAside");
      if (a) { a.addEventListener("click", goUpload); }
      if (b) { b.addEventListener("click", goUpload); }
      setStatus("Showing: " + (app.name || "Application"), versions.length + " releases", "Contributor view");
      renderSidebar();
    }).catch(renderError);
  }

  function architectureOptionsHTML(selected) {
    selected = selected || [];
    function checked(code) { return selected.indexOf(code) >= 0 ? " checked" : ""; }
    var intel32Selected = selected.indexOf("i386") >= 0 || selected.indexOf("i686") >= 0;
    var intel32Code = selected.indexOf("i686") >= 0 ? "i686" : "i386";
    return '<div class="architecture-grid">' +
      '<label class="architecture-card architecture-card-with-select"><input name="architecture_family" type="checkbox" value="intel32"' + (intel32Selected ? " checked" : "") + '><span><strong>Intel 32 Bit (i386 or i686)</strong><small>Choose the exact catalog code when known.</small>' +
      '<select name="intel32_code" aria-label="Intel 32-bit architecture code"><option value="i386"' + (intel32Code === "i386" ? " selected" : "") + '>i386 — generic Intel 32-bit</option><option value="i686"' + (intel32Code === "i686" ? " selected" : "") + '>i686 — i686-specific</option></select></span></label>' +
      '<label class="architecture-card"><input name="architecture" type="checkbox" value="x86_64"' + checked("x86_64") + '><span><strong>Intel 64 Bit (x86_64)</strong><small>64-bit Intel build.</small></span></label>' +
      '<label class="architecture-card"><input name="architecture" type="checkbox" value="ppc"' + checked("ppc") + '><span><strong>PowerPC 32 Bit (ppc)</strong><small>Generic 32-bit PowerPC build.</small></span></label>' +
      '<label class="architecture-card"><input name="architecture" type="checkbox" value="ppc-g3"' + checked("ppc-g3") + '><span><strong>PowerPC G3 (ppc-g3)</strong><small>G3-specific build.</small></span></label>' +
      '<label class="architecture-card"><input name="architecture" type="checkbox" value="ppc-g4"' + checked("ppc-g4") + '><span><strong>PowerPC G4 (ppc-g4)</strong><small>G4-specific build.</small></span></label>' +
      '<label class="architecture-card"><input name="architecture" type="checkbox" value="ppc-g5"' + checked("ppc-g5") + '><span><strong>PowerPC G5 (ppc-g5)</strong><small>G5-specific build.</small></span></label>' +
      '<label class="architecture-card"><input name="architecture" type="checkbox" value="ppc64"' + checked("ppc64") + '><span><strong>PowerPC 64 Bit (ppc64)</strong><small>64-bit PowerPC build.</small></span></label>' +
      '</div>';
  }

  function renderUploadVersion(appId) {
    if (!canUpload(state.currentUser)) { routeTo("profile"); return; }
    state.uploadContext = { appId: String(appId), versionId: "", stageUid: "", file: null, inspection: null, iconDataURL: "" };
    main.innerHTML = '<div class="loading">Loading application...</div>';
    api("/admin/apps/" + encodeURIComponent(appId)).then(function (payload) {
      renderReleaseUploadStepOne(payload.app || {});
    }).catch(renderError);
  }

  function architectureHumanLabel(code) {
    var labels = {
      i386: "Intel 32 Bit (i386 or i686)",
      i686: "Intel 32 Bit (i386 or i686)",
      x86_64: "Intel 64 Bit (x86_64)",
      ppc: "PowerPC 32 Bit (ppc)",
      "ppc-g3": "PowerPC G3 (ppc-g3)",
      "ppc-g4": "PowerPC G4 (ppc-g4)",
      "ppc-g5": "PowerPC G5 (ppc-g5)",
      ppc64: "PowerPC 64 Bit (ppc64)"
    };
    return labels[code] || code;
  }

  function humanArchitectureLabels(codes) {
    var labels = [];
    (codes || []).forEach(function (code) {
      var label = architectureHumanLabel(code);
      if (labels.indexOf(label) < 0) { labels.push(label); }
    });
    return labels;
  }

  function inspectionResultRow(label, value, found) {
    return '<div class="inspection-result-row ' + (found ? "is-found" : "is-missing") + '"><span class="inspection-result-state">' +
      (found ? "Found" : "Not found") + '</span><strong>' + escapeHTML(label) + '</strong><span>' + escapeHTML(value || "—") + '</span></div>';
  }

  function showReleaseInspectionResult(stage, inspection) {
    var node = document.getElementById("inspectionResults");
    if (!node) { return; }
    var metadata = inspection.metadata || {};
    var architectures = metadata.architectures || stage.detected_architectures || [];
    var warnings = inspection.warnings || [];
    var iconFound = !!metadata.icon_data_url;
    node.innerHTML =
      '<h3>Inspection result</h3><p class="panel-help">Review what LegacyStore could and could not determine automatically. Missing values can be entered manually on the next step.</p>' +
      '<div class="inspection-result-grid">' +
      inspectionResultRow("Application name", metadata.name || stage.detected_name, !!(metadata.name || stage.detected_name)) +
      inspectionResultRow("Bundle identifier", metadata.bundle_id || stage.detected_bundle_id, !!(metadata.bundle_id || stage.detected_bundle_id)) +
      inspectionResultRow("Version", metadata.version || stage.detected_version, !!(metadata.version || stage.detected_version)) +
      inspectionResultRow("Minimum OS X", metadata.minimum_os || stage.detected_min_os, !!(metadata.minimum_os || stage.detected_min_os)) +
      inspectionResultRow("Architectures", humanArchitectureLabels(architectures).join(", "), architectures.length > 0) +
      inspectionResultRow("Application icon", iconFound ? "Embedded icon detected" : "", iconFound) +
      '</div>' +
      (warnings.length ? '<div class="inspection-warning-list"><h4>Warnings</h4>' + warnings.map(function (warning) {
        return '<div class="web-security-note">' + escapeHTML(warning.message || warning.code || "Unknown metadata warning") + '</div>';
      }).join("") + '</div>' : '<div class="inspection-clean-note">No metadata warnings were reported.</div>');
    node.hidden = false;

    var iconNode = document.getElementById("detectedIcon");
    if (iconNode && metadata.icon_data_url) {
      iconNode.innerHTML = '<img src="' + escapeHTML(metadata.icon_data_url) + '" alt="">';
    }
    var metaNode = document.getElementById("detectedFileMeta");
    if (metaNode) {
      metaNode.textContent = formatSize(stage.size_bytes || 0) + " · SHA-256 " + (stage.sha256 || "");
    }
  }

  function renderReleaseUploadStepOne(app) {
    main.innerHTML =
      '<div class="view-title upload-page-title"><div><div class="muted">Release contribution for ' + escapeHTML(app.name || "Application") + '</div><h1>Upload Version</h1>' +
      '<p class="view-subtitle">Step 1 of 2 — upload the package, review the inspection result, then continue to release metadata.</p></div>' +
      '<button class="metal-button" data-route="manage-app/' + escapeHTML(app.id) + '" type="button">Back to Application</button></div>' +
      '<div class="contribution-steps"><div class="contribution-step is-active"><span>1</span><strong>Upload & Analyze</strong></div><div class="contribution-step"><span>2</span><strong>Release Metadata</strong></div></div>' +
      '<section class="bento-card contribution-upload-card"><div class="card-heading-row"><div><h2>Application package</h2><p class="card-kicker">DMG, PKG, ZIP or ISO</p></div><span class="status-chip" id="inspectionStatus">Waiting for file</span></div>' +
      '<input id="artifactFileInput" type="file" accept=".dmg,.pkg,.mpkg,.zip,.iso" hidden>' +
      '<button class="artifact-dropzone" id="artifactDropZone" type="button"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 16V5m0 0-4 4m4-4 4 4M5 19h14"/></svg>' +
      '<strong>Drop a release package here</strong><span>or click to choose a file</span><small>The server calculates SHA-256 and attempts to read Info.plist, version, minimum OS, icon and Mach-O architectures.</small></button>' +
      '<div class="detected-package" id="detectedPackage"><div class="detected-icon" id="detectedIcon"><span>APP</span></div><div><strong id="detectedFileName">No file selected</strong><p id="detectedFileMeta">Nothing has been uploaded yet.</p></div></div>' +
      '<div class="inspection-results" id="inspectionResults" hidden></div>' +
      '<div class="upload-submit-actions"><div id="releaseUploadNotice" class="form-message"></div>' +
      '<button class="blue-button upload-primary-button" id="analyzeReleaseButton" type="button" disabled>Upload & Analyze</button>' +
      '<button class="blue-button upload-primary-button" id="continueReleaseButton" type="button" hidden>Continue</button></div></section>' +
      '<div class="upload-drop-overlay" id="uploadDropOverlay" hidden><div class="upload-drop-overlay-card"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 16V4m0 0-4 4m4-4 4 4M4 20h16"/></svg><strong>Drop release package</strong><span>The file will stay in quarantine until Step 2 is submitted.</span></div></div>';

    bindRouteButtons(main);
    bindReleaseDropZone(app);
    setStatus("Showing: Management", "Upload Version — Step 1", app.name || "");
    renderSidebar();
  }

  function bindReleaseDropZone(app) {
    var dropZone = document.getElementById("artifactDropZone");
    var fileInput = document.getElementById("artifactFileInput");
    var overlay = document.getElementById("uploadDropOverlay");
    var analyze = document.getElementById("analyzeReleaseButton");
    var continueButton = document.getElementById("continueReleaseButton");
    if (!dropZone || !fileInput || !overlay || !analyze || !continueButton) { return; }

    function resetInspection() {
      state.uploadContext.stageUid = "";
      state.uploadContext.inspection = null;
      state.uploadContext.iconDataURL = "";
      var results = document.getElementById("inspectionResults");
      if (results) { results.hidden = true; results.innerHTML = ""; }
      continueButton.hidden = true;
      analyze.hidden = false;
    }

    function selectFile(file) {
      if (!file) { return; }
      resetInspection();
      state.uploadContext.file = file;
      var nameNode = document.getElementById("detectedFileName");
      var metaNode = document.getElementById("detectedFileMeta");
      if (nameNode) { nameNode.textContent = file.name; }
      if (metaNode) { metaNode.textContent = formatSize(file.size) + " · ready to upload"; }
      analyze.disabled = false;
      var status = document.getElementById("inspectionStatus");
      if (status) { status.textContent = "Ready"; status.classList.remove("active"); }
    }

    dropZone.addEventListener("click", function () { fileInput.click(); });
    fileInput.addEventListener("change", function () { selectFile(fileInput.files && fileInput.files[0]); });

    var dragDepth = 0;
    main.ondragenter = function (event) { event.preventDefault(); dragDepth += 1; overlay.hidden = false; };
    main.ondragover = function (event) { event.preventDefault(); if (event.dataTransfer) { event.dataTransfer.dropEffect = "copy"; } };
    main.ondragleave = function (event) { event.preventDefault(); dragDepth = Math.max(0, dragDepth - 1); if (!dragDepth) { overlay.hidden = true; } };
    main.ondrop = function (event) {
      event.preventDefault();
      dragDepth = 0;
      overlay.hidden = true;
      if (event.dataTransfer && event.dataTransfer.files && event.dataTransfer.files[0]) {
        selectFile(event.dataTransfer.files[0]);
      }
    };

    analyze.addEventListener("click", function () {
      var file = state.uploadContext.file;
      if (!file) { return; }
      analyze.disabled = true;
      var status = document.getElementById("inspectionStatus");
      if (status) { status.textContent = "Uploading & analyzing..."; }
      showMessage("releaseUploadNotice", "Uploading package to quarantine and reading metadata...", false);
      var formData = new FormData();
      formData.append("file", file, file.name);
      api("/contributions/apps/" + encodeURIComponent(app.id) + "/stage", { method: "POST", body: formData }).then(function (payload) {
        state.uploadContext.stageUid = payload.stage && payload.stage.uid ? payload.stage.uid : "";
        state.uploadContext.inspection = payload.inspection || {};
        state.uploadContext.iconDataURL = payload.inspection && payload.inspection.metadata ? payload.inspection.metadata.icon_data_url || "" : "";
        showReleaseInspectionResult(payload.stage || {}, payload.inspection || {});
        analyze.hidden = true;
        continueButton.hidden = false;
        if (status) { status.textContent = "Analysis complete"; status.classList.add("active"); }
        showMessage("releaseUploadNotice", "Inspection complete. Review the result before continuing.", false);
      }).catch(function (error) {
        analyze.disabled = false;
        if (status) { status.textContent = "Upload failed"; }
        showMessage("releaseUploadNotice", humanError(error), true);
      });
    });

    continueButton.addEventListener("click", function () {
      if (!state.uploadContext.stageUid) { return; }
      renderReleaseMetadataStep(app, {
        uid: state.uploadContext.stageUid,
        original_filename: state.uploadContext.file ? state.uploadContext.file.name : "",
        size_bytes: state.uploadContext.file ? state.uploadContext.file.size : 0,
        sha256: ""
      }, state.uploadContext.inspection || {});
      api("/contributions/uploads/" + encodeURIComponent(state.uploadContext.stageUid)).then(function (payload) {
        if (payload && payload.stage && document.getElementById("releaseMetadataForm")) {
          renderReleaseMetadataStep(app, payload.stage, state.uploadContext.inspection || {});
        }
      }).catch(function () {});
    });
  }

  function renderReleaseMetadataStep(app, stage, inspection) {
    var metadata=inspection.metadata||{};
    var detectedArch=metadata.architectures||stage.detected_architectures||[];
    var warnings=inspection.warnings||[];
    main.ondragenter=null; main.ondragover=null; main.ondragleave=null; main.ondrop=null;
    main.innerHTML =
      '<div class="view-title upload-page-title"><div><div class="muted">Release contribution for ' + escapeHTML(app.name || "Application") + '</div><h1>Review Release Metadata</h1>' +
      '<p class="view-subtitle">Step 2 of 2 — verify detected values, add release-specific metadata and submit the artifact.</p></div></div>' +
      '<div class="contribution-steps"><div class="contribution-step is-complete"><span>✓</span><strong>Upload & Analyze</strong></div><div class="contribution-step is-active"><span>2</span><strong>Release Metadata</strong></div></div>' +
      '<form id="releaseMetadataForm" class="upload-bento">' +
      '<section class="bento-card upload-package-card"><div class="card-heading-row"><div><h2>Analyzed package</h2><p class="card-kicker">' + escapeHTML(stage.original_filename || inspection.file_name || "") + '</p></div><span class="status-chip active">SHA-256 calculated</span></div>' +
      '<div class="detected-package"><div class="detected-icon">' + (metadata.icon_data_url?'<img src="'+escapeHTML(metadata.icon_data_url)+'" alt="">':'<span>APP</span>') + '</div><div><strong>' + escapeHTML(metadata.name || app.name || "Application") + '</strong>' +
      '<p>' + escapeHTML(formatSize(stage.size_bytes || 0)) + ' · <span class="mono">' + escapeHTML(stage.sha256 || "") + '</span></p></div></div>' +
      (warnings.length?'<div class="inspection-warning-list">'+warnings.map(function(w){return '<div class="web-security-note">'+escapeHTML(w.message||w.code)+'</div>';}).join("")+'</div>':'') +
      '</section>' +
      '<section class="bento-card upload-release-card"><h2>Release</h2><div class="compact-field-grid">' +
      '<div class="form-row field-version"><label>Version <span class="required-dot">*</span></label><input name="version" type="text" maxlength="64" value="' + escapeHTML(metadata.version || stage.detected_version || "") + '" required></div>' +
      '<div class="form-row field-date"><label>Release date</label><input name="release_date" type="date"></div>' +
      '<label class="switch-card"><input name="is_recommended" type="checkbox" value="true" checked><span><strong>Recommended build</strong><small>Prefer this release when it matches the selected Mac.</small></span></label>' +
      '<div class="form-row field-full"><label>Changelog</label><textarea name="changelog" placeholder="What changed in this release?"></textarea></div>' +
      '</div></section>' +
      '<section class="bento-card upload-compat-card"><h2>Compatibility</h2><p class="panel-help">Architecture and bitness are represented by the same architecture codes. There are no separate 32/64-bit flags.</p>' +
      '<div class="compact-field-grid"><div class="form-row field-os"><label>Minimum OS X <span class="required-dot">*</span></label><input name="min_os" type="text" value="' + escapeHTML(metadata.minimum_os || stage.detected_min_os || "10.4") + '" placeholder="10.6.8" required></div>' +
      '<div class="form-row field-os"><label>Maximum supported</label><input name="max_supported_os" type="text" placeholder="10.14.6"></div>' +
      '<div class="form-row field-os"><label>Maximum tested</label><input name="max_tested_os" type="text" placeholder="10.15.7"></div>' +
      '<div class="form-row field-full"><label>Install notes</label><input name="install_notes" type="text" placeholder="Requires Java 6, Rosetta, reboot, etc."></div></div>' +
      '<h3>Architectures</h3>' + architectureOptionsHTML(detectedArch) +
      '<div class="capability-grid compact-capabilities"><label><input name="requires_rosetta" type="checkbox" value="true"><span><strong>Requires Rosetta</strong><small>For translated PowerPC execution on Intel OS X.</small></span></label>' +
      '<label><input name="requires_java" type="checkbox" value="true"><span><strong>Requires Java</strong><small>Application requires a Java runtime.</small></span></label>' +
      '<label><input name="hard_block_above_max" type="checkbox" value="true"><span><strong>Hard maximum</strong><small>Block systems newer than Maximum supported.</small></span></label></div></section>' +
      '<section class="bento-card upload-media-card"><h2>Release Images</h2><p class="panel-help">These images are associated with this specific version.</p>' +
      '<div class="compact-field-grid"><div class="form-row field-wide"><label>Version-specific icon</label><input name="icon_file" type="file" accept="image/jpeg,image/png,image/gif,image/svg+xml,.svg"><small>Leave empty to use the detected application icon when available.</small></div>' +
      '<div class="form-row field-full"><label>Screenshots</label><input name="screenshots" type="file" accept="image/jpeg,image/png,image/gif" multiple><small>Up to 3 images, 2 MB each.</small></div></div></section>' +
      '<section class="bento-card upload-submit-card"><div><h2>Submit release</h2><p>The staged package will become an artifact and enter moderation according to your role.</p></div>' +
      '<div class="upload-submit-actions"><button class="metal-button" data-route="manage-app/' + escapeHTML(app.id) + '" type="button">Cancel</button><button class="blue-button upload-primary-button" id="commitReleaseButton" type="submit">Submit Release</button></div>' +
      '<div id="releaseMetadataNotice" class="form-message"></div></section></form>';

    bindRouteButtons(main);
    bindReleaseMetadataForm(app,stage);
    setStatus("Showing: Management","Upload Version — Step 2",app.name||"");
    renderSidebar();
  }

  function selectedArchitectures(form) {
    var values = Array.prototype.slice.call(form.querySelectorAll('[name="architecture"]:checked')).map(function(input){return input.value;});
    var intel32 = form.querySelector('[name="architecture_family"][value="intel32"]');
    var intel32Code = form.querySelector('[name="intel32_code"]');
    if (intel32 && intel32.checked) {
      values.unshift(intel32Code && intel32Code.value === "i686" ? "i686" : "i386");
    }
    return values;
  }

  function dataURLToBlob(dataURL) {
    var parts = String(dataURL || "").split(",");
    if (parts.length !== 2) { throw new Error("invalid_image_data"); }
    var mimeMatch = parts[0].match(/^data:([^;]+);base64$/i);
    if (!mimeMatch) { throw new Error("invalid_image_data"); }
    var binary = atob(parts[1]);
    var bytes = new Uint8Array(binary.length);
    for (var i = 0; i < binary.length; i += 1) { bytes[i] = binary.charCodeAt(i); }
    return new Blob([bytes], { type: mimeMatch[1] });
  }

  function uploadReleaseIcon(appId, versionId, form) {
    var input=form.querySelector('[name="icon_file"]');
    var file=input&&input.files?input.files[0]:null;
    var formData=new FormData();
    if(file){
      formData.append("file",file,file.name||"icon");
    }else{
      var inspection=state.uploadContext.inspection||{};
      var metadata=inspection.metadata||{};
      if(!metadata.icon_data_url){return Promise.resolve();}
      var blob=dataURLToBlob(metadata.icon_data_url);
      formData.append("file",blob,"detected-icon.png");
    }
    formData.append("app_version_id",String(versionId));
    formData.append("min_os",form.querySelector('[name="min_os"]').value||"10.4");
    formData.append("max_os",form.querySelector('[name="max_supported_os"]').value||form.querySelector('[name="max_tested_os"]').value||"15");
    return api("/admin/apps/"+encodeURIComponent(appId)+"/icons/upload",{method:"POST",body:formData});
  }

  function uploadReleaseScreenshots(appId, versionId, form) {
    var input=form.querySelector('[name="screenshots"]');
    var files=input&&input.files?Array.prototype.slice.call(input.files):[];
    if(!files.length){return Promise.resolve();}
    var formData=new FormData();
    files.forEach(function(file){formData.append("images",file,file.name);});
    formData.append("app_version_id",String(versionId));
    formData.append("min_os",form.querySelector('[name="min_os"]').value||"10.4");
    formData.append("max_os",form.querySelector('[name="max_supported_os"]').value||form.querySelector('[name="max_tested_os"]').value||"15");
    return api("/admin/apps/"+encodeURIComponent(appId)+"/screenshots/upload",{method:"POST",body:formData});
  }

  function validateReleaseImages(form) {
    var iconInput=form.querySelector('[name="icon_file"]');
    var icon=iconInput&&iconInput.files?iconInput.files[0]:null;
    if(icon&&icon.size>2*1024*1024){return "Version-specific icon must be 2 MB or smaller.";}
    var screenshotsInput=form.querySelector('[name="screenshots"]');
    var screenshots=screenshotsInput&&screenshotsInput.files?Array.prototype.slice.call(screenshotsInput.files):[];
    if(screenshots.length>3){return "You can upload at most 3 screenshots for a release.";}
    if(screenshots.some(function(file){return file.size>2*1024*1024;})){return "Each screenshot must be 2 MB or smaller.";}
    return "";
  }

  function bindReleaseMetadataForm(app, stage) {
    var form = document.getElementById("releaseMetadataForm");
    if (!form) { return; }
    form.addEventListener("submit", function (event) {
      event.preventDefault();
      var imageError = validateReleaseImages(form);
      if (imageError) { showMessage("releaseMetadataNotice", imageError, true); return; }
      var architectures = selectedArchitectures(form);
      if (!architectures.length) { showMessage("releaseMetadataNotice", "Select at least one architecture.", true); return; }
      var button = document.getElementById("commitReleaseButton");
      button.disabled = true;
      showMessage("releaseMetadataNotice", "Creating release and artifact...", false);
      var data = formJSON(form);
      jsonRequest("POST", "/contributions/uploads/" + encodeURIComponent(stage.uid || state.uploadContext.stageUid) + "/commit", {
        version: data.version,
        release_date: data.release_date || "",
        changelog: data.changelog || "",
        is_recommended: !!form.querySelector('[name="is_recommended"]:checked'),
        min_os: data.min_os,
        max_supported_os: data.max_supported_os || "",
        max_tested_os: data.max_tested_os || "",
        hard_block_above_max: !!form.querySelector('[name="hard_block_above_max"]:checked'),
        architectures: architectures,
        requires_rosetta: !!form.querySelector('[name="requires_rosetta"]:checked'),
        requires_java: !!form.querySelector('[name="requires_java"]:checked'),
        install_notes: data.install_notes || ""
      }).then(function (payload) {
        var release = payload.release || {};
        state.uploadContext.versionId = String(release.version_id || "");
        showMessage("releaseMetadataNotice", "Release created. Processing version-specific images...", false);
        return uploadReleaseIcon(app.id, release.version_id, form)
          .then(function () { return uploadReleaseScreenshots(app.id, release.version_id, form); })
          .then(function () { return release; });
      }).then(function (release) {
        if (release.moderation_status === "pending") {
          showToast("Release submitted. It will become public after moderation.", "success");
        } else {
          showToast("Release published.", "success");
        }
        routeTo("manage-app/" + encodeURIComponent(app.id));
      }).catch(function (error) {
        button.disabled = false;
        showMessage("releaseMetadataNotice", humanError(error), true);
      });
    });
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
    stopHomeCarousel();
    main.classList.remove("is-auth");
    setActiveTabs();

    if (state.route === "home") { renderHome(); return; }
    if (state.route === "catalog") { renderCatalog("", parsed.params.get("sort") || ""); return; }
    if (state.route.indexOf("category/") === 0) { renderCatalog(state.route.split("/")[1], ""); return; }
    if (state.route === "categories") { renderCategories(); return; }
    if (state.route === "search") { renderSearch(parsed.params.get("q") || ""); return; }
    if (state.route.indexOf("app/") === 0) { renderApp(state.route.split("/")[1]); return; }
    if (state.route === "top-charts") { renderTopCharts(); return; }
    if (state.route.indexOf("manage-app/") === 0) {
      if (!canUpload(state.currentUser)) { routeTo(state.currentUser ? "profile" : "login"); return; }
      renderContributorApp(state.route.split("/")[1]); return;
    }
    if (state.route.indexOf("upload-version/") === 0) {
      if (!canUpload(state.currentUser)) { routeTo(state.currentUser ? "profile" : "login"); return; }
      renderUploadVersion(state.route.split("/")[1]); return;
    }
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
