package webui

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"legacystore/backend/internal/account"
	"legacystore/backend/internal/catalog"
	"legacystore/backend/internal/compatibility"
	"legacystore/backend/internal/config"
)

//go:embed templates/*.html assets/*
var embeddedFiles embed.FS

type Handler struct {
	cfg       config.Config
	catalog   *catalog.Store
	users     *account.Store
	api       http.Handler
	mux       *http.ServeMux
	static    http.Handler
	templates map[string]*template.Template
}

type PageData struct {
	Title            string
	Active           string
	User             *account.User
	Session          *account.Session
	Home             *catalog.HomeFeed
	Hero             *catalog.HomeSlide
	Apps             []catalog.AppSummary
	Categories       []catalog.Category
	App              *catalog.AppDetail
	AppReviews       []catalog.Review
	Query            string
	IsSearch         bool
	Mode             string
	Error            string
	Notice           string
	RecoveryToken    string
	Sessions         []account.Session
	Dashboard        map[string]int
	Moderation       []account.ModerationItem
	ModerationStatus string
	SystemTab        string
	Users            []account.User
	AdminApps        []account.AdminApp
	Artifacts        []account.AdminArtifactEntry
	Reviews          []account.AdminReviewEntry
	Audit            []map[string]string
}

func New(cfg config.Config, catalogStore *catalog.Store, userStore *account.Store, apiHandler http.Handler) (http.Handler, error) {
	staticFS, err := fs.Sub(embeddedFiles, "assets")
	if err != nil {
		return nil, fmt.Errorf("web assets: %w", err)
	}
	h := &Handler{
		cfg:       cfg,
		catalog:   catalogStore,
		users:     userStore,
		api:       apiHandler,
		mux:       http.NewServeMux(),
		static:    http.StripPrefix("/assets/", http.FileServer(http.FS(staticFS))),
		templates: make(map[string]*template.Template),
	}
	funcs := template.FuncMap{
		"hasRole": func(user *account.User, role string) bool {
			return user != nil && account.HasRole(*user, role)
		},
		"roles": func(values []string) string { return strings.Join(values, ", ") },
		"join":  func(values []string) string { return strings.Join(values, ",") },
		"initial": func(value string) string {
			value = strings.TrimSpace(value)
			if value == "" {
				return "?"
			}
			runes := []rune(value)
			return strings.ToUpper(string(runes[0]))
		},
	}
	for _, page := range []string{"home", "top_charts", "categories", "app", "auth", "profile", "admin_dashboard", "admin_moderation", "admin_system", "error"} {
		t, err := template.New("layout.html").Funcs(funcs).ParseFS(embeddedFiles, "templates/layout.html", "templates/"+page+".html")
		if err != nil {
			return nil, fmt.Errorf("parse %s template: %w", page, err)
		}
		h.templates[page] = t
	}
	h.routes()
	return h, nil
}

func (h *Handler) routes() {
	h.mux.HandleFunc("GET /", h.home)
	h.mux.HandleFunc("GET /top-charts", h.topCharts)
	h.mux.HandleFunc("GET /categories", h.categories)
	h.mux.HandleFunc("GET /search", h.search)
	h.mux.HandleFunc("GET /app/{slug}", h.appDetail)
	h.mux.HandleFunc("GET /download/{id}", h.download)
	h.mux.HandleFunc("GET /account", h.authPage)
	h.mux.HandleFunc("POST /account/login", h.login)
	h.mux.HandleFunc("POST /account/register", h.register)
	h.mux.HandleFunc("POST /account/recovery", h.recovery)
	h.mux.HandleFunc("POST /account/reset", h.resetPassword)
	h.mux.HandleFunc("POST /account/logout", h.logout)
	h.mux.HandleFunc("GET /account/profile", h.profile)
	h.mux.HandleFunc("POST /account/profile", h.updateProfile)
	h.mux.HandleFunc("POST /account/profile/sessions/{id}/revoke", h.revokeSession)
	h.mux.HandleFunc("GET /admin", h.adminDashboard)
	h.mux.HandleFunc("GET /admin/moderation", h.adminModeration)
	h.mux.HandleFunc("POST /admin/moderation/{id}/approve", h.adminApprove)
	h.mux.HandleFunc("POST /admin/moderation/{id}/reject", h.adminReject)
	h.mux.HandleFunc("GET /admin/system", h.adminSystem)
	h.registerAdminSystemRoutes()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/healthz" || strings.HasPrefix(req.URL.Path, "/api/") {
		h.api.ServeHTTP(w, req)
		return
	}
	if strings.HasPrefix(req.URL.Path, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		h.static.ServeHTTP(w, req)
		return
	}
	h.mux.ServeHTTP(w, req)
}

func (h *Handler) baseData(req *http.Request, title, active string) PageData {
	data := PageData{Title: title, Active: active}
	user, session, _ := h.currentUser(req)
	data.User = user
	data.Session = session
	data.Notice = strings.TrimSpace(req.URL.Query().Get("notice"))
	return data
}

func (h *Handler) render(w http.ResponseWriter, page string, status int, data PageData) {
	t, ok := h.templates[page]
	if !ok {
		http.Error(w, "template_not_found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		return
	}
}

func (h *Handler) renderError(w http.ResponseWriter, req *http.Request, status int, message string) {
	data := h.baseData(req, "Ошибка", "")
	data.Error = message
	h.render(w, "error", status, data)
}

func (h *Handler) currentUser(req *http.Request) (*account.User, *account.Session, error) {
	if h.users == nil {
		return nil, nil, account.ErrUnauthorized
	}
	cookie, err := req.Cookie("legacystore_session")
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return nil, nil, account.ErrUnauthorized
	}
	return h.users.UserBySession(req.Context(), cookie.Value, clientIP(req, h.cfg.TrustProxyHeaders), req.UserAgent())
}

func (h *Handler) requireUser(w http.ResponseWriter, req *http.Request) (*account.User, *account.Session, bool) {
	user, session, err := h.currentUser(req)
	if err != nil || session == nil || session.AuthKind != "web" {
		http.Redirect(w, req, "/account?mode=login&notice=auth-required", http.StatusSeeOther)
		return nil, nil, false
	}
	return user, session, true
}

func (h *Handler) requireRoles(w http.ResponseWriter, req *http.Request, roles ...string) (*account.User, *account.Session, bool) {
	user, session, ok := h.requireUser(w, req)
	if !ok {
		return nil, nil, false
	}
	if !account.HasRole(*user, roles...) {
		h.renderError(w, req, http.StatusForbidden, "Недостаточно прав для этой страницы.")
		return nil, nil, false
	}
	return user, session, true
}

func (h *Handler) secureRequest(req *http.Request) bool {
	if req.TLS != nil {
		return true
	}
	return h.cfg.TrustProxyHeaders && strings.EqualFold(strings.TrimSpace(req.Header.Get("X-Forwarded-Proto")), "https")
}

func (h *Handler) parseForm(w http.ResponseWriter, req *http.Request) bool {
	req.Body = http.MaxBytesReader(w, req.Body, 1<<20)
	if err := req.ParseForm(); err != nil {
		h.renderError(w, req, http.StatusBadRequest, "Не удалось разобрать форму.")
		return false
	}
	return true
}

func (h *Handler) home(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		h.renderError(w, req, http.StatusNotFound, "Страница не найдена.")
		return
	}
	data := h.baseData(req, "LegacyStore", "home")
	if h.catalog == nil {
		h.renderError(w, req, http.StatusServiceUnavailable, "Каталог временно недоступен.")
		return
	}
	feed, err := h.catalog.Home(req.Context(), compatibility.Target{})
	if err != nil {
		h.renderError(w, req, http.StatusInternalServerError, "Не удалось загрузить главную страницу.")
		return
	}
	data.Home = feed
	if len(feed.Slides) > 0 {
		data.Hero = &feed.Slides[0]
	}
	h.render(w, "home", http.StatusOK, data)
}

func (h *Handler) topCharts(w http.ResponseWriter, req *http.Request) {
	data := h.baseData(req, "Top Charts", "top-charts")
	if h.catalog == nil {
		h.renderError(w, req, http.StatusServiceUnavailable, "Каталог временно недоступен.")
		return
	}
	apps, err := h.catalog.Apps(req.Context(), catalog.Filters{
		Page: 1, Limit: 50, Sort: "popular",
	})
	if err != nil {
		h.renderError(w, req, http.StatusInternalServerError, "Не удалось загрузить рейтинг приложений.")
		return
	}
	data.Apps = apps
	h.render(w, "top_charts", http.StatusOK, data)
}

func (h *Handler) categories(w http.ResponseWriter, req *http.Request) {
	data := h.baseData(req, "Categories", "categories")
	if h.catalog == nil {
		h.renderError(w, req, http.StatusServiceUnavailable, "Каталог временно недоступен.")
		return
	}
	categories, err := h.catalog.Categories(req.Context())
	if err != nil {
		h.renderError(w, req, http.StatusInternalServerError, "Не удалось загрузить категории.")
		return
	}
	data.Categories = categories
	h.render(w, "categories", http.StatusOK, data)
}

func (h *Handler) search(w http.ResponseWriter, req *http.Request) {
	data := h.baseData(req, "Поиск", "search")
	if h.catalog == nil {
		h.renderError(w, req, http.StatusServiceUnavailable, "Каталог временно недоступен.")
		return
	}
	data.Query = strings.TrimSpace(req.URL.Query().Get("q"))
	data.IsSearch = true
	if data.Query != "" {
		apps, err := h.catalog.Apps(req.Context(), catalog.Filters{
			Page: 1, Limit: 48, Query: data.Query, Sort: strings.TrimSpace(req.URL.Query().Get("sort")),
		})
		if err != nil {
			h.renderError(w, req, http.StatusInternalServerError, "Поиск не удался.")
			return
		}
		data.Apps = apps
	}
	h.render(w, "home", http.StatusOK, data)
}

func (h *Handler) appDetail(w http.ResponseWriter, req *http.Request) {
	data := h.baseData(req, "Приложение", "categories")
	if h.catalog == nil {
		h.renderError(w, req, http.StatusServiceUnavailable, "Каталог временно недоступен.")
		return
	}
	app, err := h.catalog.AppBySlug(req.Context(), req.PathValue("slug"), compatibility.Target{})
	if errors.Is(err, catalog.ErrNotFound) {
		h.renderError(w, req, http.StatusNotFound, "Приложение не найдено.")
		return
	}
	if err != nil {
		h.renderError(w, req, http.StatusInternalServerError, "Не удалось загрузить приложение.")
		return
	}
	reviews, err := h.catalog.Reviews(req.Context(), app.Slug)
	if err != nil && !errors.Is(err, catalog.ErrNotFound) {
		h.renderError(w, req, http.StatusInternalServerError, "Не удалось загрузить отзывы.")
		return
	}
	data.Title = app.Name
	data.App = app
	data.AppReviews = reviews
	h.render(w, "app", http.StatusOK, data)
}

func (h *Handler) download(w http.ResponseWriter, req *http.Request) {
	if h.catalog == nil {
		h.renderError(w, req, http.StatusServiceUnavailable, "Каталог временно недоступен.")
		return
	}
	id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if err != nil {
		h.renderError(w, req, http.StatusBadRequest, "Некорректный идентификатор загрузки.")
		return
	}
	item, err := h.catalog.Download(req.Context(), id, compatibility.Target{})
	if errors.Is(err, catalog.ErrNotFound) {
		h.renderError(w, req, http.StatusNotFound, "Файл не найден.")
		return
	}
	if err != nil {
		h.renderError(w, req, http.StatusInternalServerError, "Не удалось подготовить загрузку.")
		return
	}

	target := strings.TrimSpace(item.DownloadURL)
	switch item.SourceType {
	case "local":
		target = "/api/v1/files/" + strconv.FormatInt(id, 10)
	case "external_page":
		target = strings.TrimSpace(item.ExternalPageURL)
	}
	if target == "" {
		if item.TorrentURL != "" {
			target = item.TorrentURL
		} else if item.MagnetURL != "" {
			target = item.MagnetURL
		}
	}
	if target == "" {
		h.renderError(w, req, http.StatusNotFound, "Для этого файла не настроен источник загрузки.")
		return
	}
	http.Redirect(w, req, target, http.StatusSeeOther)
}

func (h *Handler) authPage(w http.ResponseWriter, req *http.Request) {
	if user, _, _ := h.currentUser(req); user != nil {
		http.Redirect(w, req, "/account/profile", http.StatusSeeOther)
		return
	}
	data := h.baseData(req, "Авторизация", "account")
	data.Mode = authMode(req)
	data.RecoveryToken = strings.TrimSpace(req.URL.Query().Get("token"))
	h.render(w, "auth", http.StatusOK, data)
}

func (h *Handler) login(w http.ResponseWriter, req *http.Request) {
	if !h.secureRequest(req) {
		h.renderError(w, req, http.StatusForbidden, "Авторизация разрешена только по HTTPS.")
		return
	}
	if !h.parseForm(w, req) || h.users == nil {
		return
	}
	remember := formBool(req.FormValue("remember_me"))
	result, err := h.users.LoginWithSecondFactor(
		req.Context(),
		req.FormValue("email"),
		req.FormValue("password"),
		req.FormValue("totp_code"),
		req.FormValue("recovery_code"),
		remember,
		clientIP(req, h.cfg.TrustProxyHeaders),
		req.UserAgent(),
	)
	if err != nil {
		data := h.baseData(req, "Авторизация", "account")
		data.Mode = "login"
		switch {
		case errors.Is(err, account.ErrTwoFactorRequired):
			data.Error = "Требуется код двухфакторной аутентификации или recovery-код."
		case errors.Is(err, account.ErrInvalidCredential):
			data.Error = "Неверный email, пароль или код подтверждения."
		default:
			data.Error = "Не удалось выполнить вход."
		}
		h.render(w, "auth", http.StatusUnauthorized, data)
		return
	}
	setSessionCookie(w, result.Token, result.ExpiresAt, remember)
	http.Redirect(w, req, "/", http.StatusSeeOther)
}

func (h *Handler) register(w http.ResponseWriter, req *http.Request) {
	if !h.secureRequest(req) {
		h.renderError(w, req, http.StatusForbidden, "Регистрация разрешена только по HTTPS.")
		return
	}
	if !h.parseForm(w, req) || h.users == nil {
		return
	}
	remember := formBool(req.FormValue("remember_me"))
	result, err := h.users.Register(
		req.Context(),
		req.FormValue("email"),
		req.FormValue("nickname"),
		req.FormValue("password"),
		remember,
		clientIP(req, h.cfg.TrustProxyHeaders),
		req.UserAgent(),
	)
	if err != nil {
		data := h.baseData(req, "Регистрация", "account")
		data.Mode = "register"
		if errors.Is(err, account.ErrEmailExists) {
			data.Error = "Аккаунт с таким email уже существует."
		} else {
			data.Error = "Не удалось создать аккаунт. Пароль должен содержать не менее 8 символов."
		}
		h.render(w, "auth", http.StatusBadRequest, data)
		return
	}
	setSessionCookie(w, result.Token, result.ExpiresAt, remember)
	http.Redirect(w, req, "/account/profile?notice=registered", http.StatusSeeOther)
}

func (h *Handler) recovery(w http.ResponseWriter, req *http.Request) {
	if !h.secureRequest(req) {
		h.renderError(w, req, http.StatusForbidden, "Восстановление пароля разрешено только по HTTPS.")
		return
	}
	if !h.parseForm(w, req) || h.users == nil {
		return
	}
	data := h.baseData(req, "Восстановление пароля", "account")
	data.Mode = "recovery"
	if h.cfg.SMTPHost == "" && !h.cfg.RecoveryDebugToken {
		data.Error = "Доставка писем восстановления на сервере пока не настроена."
		h.render(w, "auth", http.StatusServiceUnavailable, data)
		return
	}
	token, found, err := h.users.BeginPasswordRecovery(req.Context(), req.FormValue("email"))
	if err != nil {
		data.Error = "Не удалось создать запрос восстановления."
		h.render(w, "auth", http.StatusInternalServerError, data)
		return
	}
	if found && h.cfg.SMTPHost != "" {
		_ = sendRecoveryEmail(h.cfg, strings.TrimSpace(req.FormValue("email")), token)
	}
	data.Notice = "Если такой аккаунт существует, ссылка для восстановления отправлена."
	if found && h.cfg.RecoveryDebugToken {
		data.RecoveryToken = token
		data.Mode = "reset"
	}
	h.render(w, "auth", http.StatusAccepted, data)
}

func (h *Handler) resetPassword(w http.ResponseWriter, req *http.Request) {
	if !h.secureRequest(req) {
		h.renderError(w, req, http.StatusForbidden, "Сброс пароля разрешён только по HTTPS.")
		return
	}
	if !h.parseForm(w, req) || h.users == nil {
		return
	}
	err := h.users.CompletePasswordRecovery(req.Context(), req.FormValue("token"), req.FormValue("new_password"))
	if err != nil {
		data := h.baseData(req, "Новый пароль", "account")
		data.Mode = "reset"
		data.RecoveryToken = req.FormValue("token")
		data.Error = "Ссылка недействительна, истекла или новый пароль слишком короткий."
		h.render(w, "auth", http.StatusBadRequest, data)
		return
	}
	clearSessionCookie(w)
	http.Redirect(w, req, "/account?mode=login&notice=password-reset", http.StatusSeeOther)
}

func (h *Handler) logout(w http.ResponseWriter, req *http.Request) {
	if h.users != nil {
		if cookie, err := req.Cookie("legacystore_session"); err == nil {
			_ = h.users.Logout(req.Context(), cookie.Value)
		}
	}
	clearSessionCookie(w)
	http.Redirect(w, req, "/", http.StatusSeeOther)
}

func (h *Handler) profile(w http.ResponseWriter, req *http.Request) {
	user, session, ok := h.requireUser(w, req)
	if !ok {
		return
	}
	data := h.baseData(req, "Профиль", "profile")
	data.User = user
	data.Session = session
	sessions, err := h.users.ListSessions(req.Context(), user.ID)
	if err != nil {
		h.renderError(w, req, http.StatusInternalServerError, "Не удалось загрузить активные сессии.")
		return
	}
	data.Sessions = sessions
	h.render(w, "profile", http.StatusOK, data)
}

func (h *Handler) updateProfile(w http.ResponseWriter, req *http.Request) {
	user, _, ok := h.requireUser(w, req)
	if !ok || !h.parseForm(w, req) {
		return
	}
	_, err := h.users.UpdateProfile(req.Context(), user.ID, req.FormValue("nickname"), req.FormValue("avatar_url"))
	if err != nil {
		h.renderError(w, req, http.StatusBadRequest, "Не удалось обновить профиль.")
		return
	}
	http.Redirect(w, req, "/account/profile?notice=profile-saved", http.StatusSeeOther)
}

func (h *Handler) revokeSession(w http.ResponseWriter, req *http.Request) {
	user, session, ok := h.requireUser(w, req)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if err != nil {
		h.renderError(w, req, http.StatusBadRequest, "Некорректный идентификатор сессии.")
		return
	}
	if err := h.users.RevokeSession(req.Context(), user.ID, id); err != nil {
		h.renderError(w, req, http.StatusBadRequest, "Не удалось закрыть сессию.")
		return
	}
	if session.ID == id {
		clearSessionCookie(w)
		http.Redirect(w, req, "/account?mode=login", http.StatusSeeOther)
		return
	}
	http.Redirect(w, req, "/account/profile?notice=session-revoked", http.StatusSeeOther)
}

func (h *Handler) adminDashboard(w http.ResponseWriter, req *http.Request) {
	_, _, ok := h.requireRoles(w, req, "moder", "admin")
	if !ok {
		return
	}
	data := h.baseData(req, "Администрирование", "admin")
	dashboard, err := h.users.Dashboard(req.Context())
	if err != nil {
		h.renderError(w, req, http.StatusInternalServerError, "Не удалось загрузить dashboard.")
		return
	}
	data.Dashboard = dashboard
	h.render(w, "admin_dashboard", http.StatusOK, data)
}

func (h *Handler) adminModeration(w http.ResponseWriter, req *http.Request) {
	_, _, ok := h.requireRoles(w, req, "moder", "admin")
	if !ok {
		return
	}
	data := h.baseData(req, "Модерация", "moderation")
	data.ModerationStatus = strings.TrimSpace(req.URL.Query().Get("status"))
	if data.ModerationStatus == "" {
		data.ModerationStatus = "pending"
	}
	items, err := h.users.ListModeration(req.Context(), data.ModerationStatus)
	if err != nil {
		h.renderError(w, req, http.StatusInternalServerError, "Не удалось загрузить очередь модерации.")
		return
	}
	data.Moderation = items
	h.render(w, "admin_moderation", http.StatusOK, data)
}

func (h *Handler) adminApprove(w http.ResponseWriter, req *http.Request) {
	h.adminModerate(w, req, true)
}

func (h *Handler) adminReject(w http.ResponseWriter, req *http.Request) {
	h.adminModerate(w, req, false)
}

func (h *Handler) adminModerate(w http.ResponseWriter, req *http.Request, approve bool) {
	user, _, ok := h.requireRoles(w, req, "moder", "admin")
	if !ok || !h.parseForm(w, req) {
		return
	}
	id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if err != nil {
		h.renderError(w, req, http.StatusBadRequest, "Некорректная запись модерации.")
		return
	}
	if err := h.users.Moderate(req.Context(), *user, id, approve, req.FormValue("comment"), clientIP(req, h.cfg.TrustProxyHeaders), req.UserAgent()); err != nil {
		h.renderError(w, req, http.StatusBadRequest, "Не удалось изменить статус модерации.")
		return
	}
	http.Redirect(w, req, "/admin/moderation?status=pending&notice=moderated", http.StatusSeeOther)
}

func (h *Handler) adminSystem(w http.ResponseWriter, req *http.Request) {
	_, _, ok := h.requireRoles(w, req, "admin")
	if !ok {
		return
	}
	h.renderAdminSystem(w, req)
}

func authMode(req *http.Request) string {
	if strings.TrimSpace(req.URL.Query().Get("token")) != "" {
		return "reset"
	}
	switch req.URL.Query().Get("mode") {
	case "register", "recovery", "reset":
		return req.URL.Query().Get("mode")
	default:
		return "login"
	}
}

func formBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time, remember bool) {
	cookie := &http.Cookie{
		Name:     "legacystore_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
	if remember {
		cookie.Expires = expiresAt
		cookie.MaxAge = int(time.Until(expiresAt).Seconds())
	}
	http.SetCookie(w, cookie)
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "legacystore_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clientIP(req *http.Request, trustProxy bool) net.IP {
	if trustProxy {
		if forwarded := req.Header.Get("X-Forwarded-For"); forwarded != "" {
			first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
			if ip := net.ParseIP(first); ip != nil {
				return ip
			}
		}
		if real := strings.TrimSpace(req.Header.Get("X-Real-IP")); real != "" {
			if ip := net.ParseIP(real); ip != nil {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil {
		return net.ParseIP(host)
	}
	return net.ParseIP(req.RemoteAddr)
}

func redirectNotice(w http.ResponseWriter, req *http.Request, path, notice string) {
	values := url.Values{}
	values.Set("notice", notice)
	http.Redirect(w, req, path+"?"+values.Encode(), http.StatusSeeOther)
}
