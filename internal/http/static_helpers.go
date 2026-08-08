package http

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"net"
	stdhttp "net/http"
	"net/url"
	"os"
	urlpath "path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	appdb "fluffcatch/internal/db"
	"fluffcatch/internal/gallery"
	"fluffcatch/internal/settings"
	frontend "fluffcatch/www"

	"github.com/gin-gonic/gin"
	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const viewerCookieName = "fluffcatch_viewer"
const viewerCookieMaxAge = 365 * 24 * 60 * 60

func (server *Server) mountStaticApp(r *gin.Engine) {
	switch server.cfg.Frontend.Mode {
	case "external", "disabled":
		return
	case "embedded":
		if dist, ok := frontend.Dist(); ok {
			server.mountStaticAppFS(r, dist)
		}
		return
	case "disk":
		server.mountDiskStaticApp(r)
		return
	case "auto":
		if dist, ok := frontend.Dist(); ok && server.mountStaticAppFS(r, dist) {
			return
		}
		server.mountDiskStaticApp(r)
	}
}

func (server *Server) mountDiskStaticApp(r *gin.Engine) {
	staticRoot := server.cfg.Frontend.StaticRoot
	indexPath := filepath.Join(staticRoot, "index.html")
	if _, err := os.Stat(indexPath); errors.Is(err, os.ErrNotExist) {
		return
	}

	fileServer := stdhttp.FileServer(stdhttp.Dir(staticRoot))
	r.NoRoute(func(c *gin.Context) {
		w := c.Writer
		request := c.Request
		cleanPath := urlpath.Clean(request.URL.Path)
		if strings.HasPrefix(cleanPath, "/api/") {
			writeError(w, stdhttp.StatusNotFound, "api route not found")
			return
		}

		targetPath := filepath.Join(staticRoot, filepath.FromSlash(strings.TrimPrefix(cleanPath, "/")))
		if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, request)
			return
		}

		content, err := os.ReadFile(indexPath)
		if err != nil {
			stdhttp.ServeFile(w, request, indexPath)
			return
		}
		server.servePrerenderedIndex(w, request, content)
	})
}

func (server *Server) mountStaticAppFS(r *gin.Engine, dist fs.FS) bool {
	indexContent, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		return false
	}

	fileServer := stdhttp.FileServer(stdhttp.FS(dist))
	r.NoRoute(func(c *gin.Context) {
		w := c.Writer
		request := c.Request
		cleanPath := urlpath.Clean(request.URL.Path)
		if strings.HasPrefix(cleanPath, "/api/") {
			writeError(w, stdhttp.StatusNotFound, "api route not found")
			return
		}

		target := strings.TrimPrefix(cleanPath, "/")
		if target != "" {
			if info, err := fs.Stat(dist, target); err == nil && !info.IsDir() {
				fileServer.ServeHTTP(w, request)
				return
			}
		}

		server.servePrerenderedIndex(w, request, indexContent)
	})

	return true
}

func (server *Server) servePrerenderedIndex(w stdhttp.ResponseWriter, r *stdhttp.Request, indexContent []byte) {
	content := server.prerenderIndexHTML(r, indexContent)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	_, _ = w.Write(content)
}

func (server *Server) prerenderIndexHTML(r *stdhttp.Request, indexContent []byte) []byte {
	site := settings.FromConfig(server.cfg).Site
	if server.settingsService != nil {
		if current, err := server.settingsService.Load(r.Context()); err == nil {
			site = current.Site
		}
	}
	siteName := siteDisplayName(site)
	pageName := server.seoPageName(r)
	title := seoDocumentTitle(pageName, siteName)
	description := strings.TrimSpace(site.Subtitle)
	logoURL := absolutePublicURL(r, strings.TrimSpace(site.LogoURL))
	pageURL := absolutePublicURL(r, r.URL.RequestURI())

	head := buildSEOHead(title, siteName, description, logoURL, pageURL)
	footer := buildSEOFooter(site)
	content := bytes.ReplaceAll(indexContent, []byte("<!--FLUFFCATCH_TITLE-->FluffCatch<!--/FLUFFCATCH_TITLE-->"), []byte(html.EscapeString(title)))
	content = bytes.ReplaceAll(content, []byte("<!--FLUFFCATCH_SEO_HEAD-->"), []byte(head))
	content = bytes.ReplaceAll(content, []byte("<!--FLUFFCATCH_SEO_FOOTER-->"), []byte(footer))
	return content
}

func buildSEOHead(title string, siteName string, description string, logoURL string, pageURL string) string {
	var builder strings.Builder
	builder.WriteString(`<meta name="application-name" content="`)
	builder.WriteString(html.EscapeString(siteName))
	builder.WriteString(`" />` + "\n")
	builder.WriteString(`    <meta property="og:title" content="`)
	builder.WriteString(html.EscapeString(title))
	builder.WriteString(`" />` + "\n")
	builder.WriteString(`    <meta property="og:site_name" content="`)
	builder.WriteString(html.EscapeString(siteName))
	builder.WriteString(`" />` + "\n")
	builder.WriteString(`    <meta property="og:type" content="website" />` + "\n")
	if description != "" {
		builder.WriteString(`    <meta name="description" content="`)
		builder.WriteString(html.EscapeString(description))
		builder.WriteString(`" />` + "\n")
		builder.WriteString(`    <meta property="og:description" content="`)
		builder.WriteString(html.EscapeString(description))
		builder.WriteString(`" />` + "\n")
	}
	if pageURL != "" {
		builder.WriteString(`    <meta property="og:url" content="`)
		builder.WriteString(html.EscapeString(pageURL))
		builder.WriteString(`" />` + "\n")
	}
	builder.WriteString(`    <meta name="twitter:card" content="summary" />`)
	if logoURL != "" {
		escapedLogo := html.EscapeString(logoURL)
		builder.WriteString("\n")
		builder.WriteString(`    <link rel="icon" href="`)
		builder.WriteString(escapedLogo)
		builder.WriteString(`" />` + "\n")
		builder.WriteString(`    <link rel="apple-touch-icon" href="`)
		builder.WriteString(escapedLogo)
		builder.WriteString(`" />` + "\n")
		builder.WriteString(`    <meta property="og:image" content="`)
		builder.WriteString(escapedLogo)
		builder.WriteString(`" />`)
	}
	return builder.String()
}

func buildSEOFooter(site settings.SiteSettings) string {
	sections := normalizedSEOFooterSections(site)
	siteName := siteDisplayName(site)
	subtitle := strings.TrimSpace(site.Subtitle)
	if len(sections) == 0 && siteName == "" && subtitle == "" {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(`<noscript><footer data-seo-footer="true"><strong>`)
	builder.WriteString(html.EscapeString(siteName))
	builder.WriteString(`</strong>`)
	if subtitle != "" {
		builder.WriteString(`<p>`)
		builder.WriteString(html.EscapeString(subtitle))
		builder.WriteString(`</p>`)
	}
	for _, section := range sections {
		builder.WriteString(`<section>`)
		if section.Title != "" {
			builder.WriteString(`<h2>`)
			builder.WriteString(html.EscapeString(section.Title))
			builder.WriteString(`</h2>`)
		}
		builder.WriteString(sanitizeSEOHTMLFragment(section.HTML))
		builder.WriteString(`</section>`)
	}
	builder.WriteString(`</footer></noscript>`)
	return builder.String()
}

func normalizedSEOFooterSections(site settings.SiteSettings) []settings.FooterSection {
	sections := site.FooterSections
	result := make([]settings.FooterSection, 0, 3)
	for _, section := range sections {
		title := strings.TrimSpace(section.Title)
		body := strings.TrimSpace(section.HTML)
		if title == "" && body == "" {
			continue
		}
		result = append(result, settings.FooterSection{Title: title, HTML: body})
		if len(result) == 3 {
			break
		}
	}
	return result
}

func sanitizeSEOHTMLFragment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	context := &xhtml.Node{Type: xhtml.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := xhtml.ParseFragment(strings.NewReader(value), context)
	if err != nil {
		return html.EscapeString(value)
	}

	root := &xhtml.Node{Type: xhtml.ElementNode, Data: "div"}
	for _, node := range nodes {
		root.AppendChild(node)
	}
	sanitizeSEOChildren(root)

	var builder strings.Builder
	for node := root.FirstChild; node != nil; node = node.NextSibling {
		_ = xhtml.Render(&builder, node)
	}
	return builder.String()
}

func sanitizeSEOChildren(parent *xhtml.Node) {
	for node := parent.FirstChild; node != nil; {
		next := node.NextSibling
		switch node.Type {
		case xhtml.TextNode:
		case xhtml.CommentNode:
			parent.RemoveChild(node)
		case xhtml.ElementNode:
			tag := strings.ToLower(node.Data)
			if blockedSEOHTMLTags[tag] {
				parent.RemoveChild(node)
				break
			}
			if !allowedSEOHTMLTags[tag] {
				sanitizeSEOChildren(node)
				unwrapSEOElement(parent, node)
				break
			}
			node.Data = tag
			sanitizeSEOAttributes(node)
			if tag == "img" && getSEOAttribute(node, "src") == "" {
				parent.RemoveChild(node)
				break
			}
			sanitizeSEOChildren(node)
		default:
			parent.RemoveChild(node)
		}
		node = next
	}
}

func unwrapSEOElement(parent *xhtml.Node, node *xhtml.Node) {
	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		node.RemoveChild(child)
		parent.InsertBefore(child, node)
		child = next
	}
	parent.RemoveChild(node)
}

func sanitizeSEOAttributes(node *xhtml.Node) {
	tag := strings.ToLower(node.Data)
	attrs := make([]xhtml.Attribute, 0, len(node.Attr))
	for _, attr := range node.Attr {
		key := strings.ToLower(strings.TrimSpace(attr.Key))
		if key == "" || key == "style" || strings.HasPrefix(key, "on") {
			continue
		}
		if !isAllowedSEOAttribute(tag, key) {
			continue
		}
		value := strings.TrimSpace(attr.Val)
		if (tag == "a" && key == "href") || (tag == "img" && key == "src") {
			if !isAllowedSEOURL(value) {
				continue
			}
		}
		if tag == "a" && key == "target" && value != "_blank" {
			continue
		}
		attrs = append(attrs, xhtml.Attribute{Key: key, Val: value})
	}
	node.Attr = attrs

	if tag == "a" && getSEOAttribute(node, "target") == "_blank" {
		setSEOAttribute(node, "rel", "noopener noreferrer")
	}
}

func isAllowedSEOAttribute(tag string, key string) bool {
	if allowedSEOGlobalAttrs[key] {
		return true
	}
	if attrs, ok := allowedSEOAttrs[tag]; ok {
		return attrs[key]
	}
	return false
}

func isAllowedSEOURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "/") {
		return !strings.HasPrefix(value, "//")
	}
	if strings.HasPrefix(value, "#") {
		return true
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "mailto", "tel":
		return true
	default:
		return false
	}
}

func getSEOAttribute(node *xhtml.Node, key string) string {
	key = strings.ToLower(key)
	for _, attr := range node.Attr {
		if strings.ToLower(attr.Key) == key {
			return attr.Val
		}
	}
	return ""
}

func setSEOAttribute(node *xhtml.Node, key string, value string) {
	key = strings.ToLower(key)
	for index, attr := range node.Attr {
		if strings.ToLower(attr.Key) == key {
			node.Attr[index].Val = value
			return
		}
	}
	node.Attr = append(node.Attr, xhtml.Attribute{Key: key, Val: value})
}

var allowedSEOHTMLTags = map[string]bool{
	"a":      true,
	"b":      true,
	"br":     true,
	"code":   true,
	"em":     true,
	"h1":     true,
	"h2":     true,
	"h3":     true,
	"h4":     true,
	"h5":     true,
	"h6":     true,
	"i":      true,
	"img":    true,
	"li":     true,
	"ol":     true,
	"p":      true,
	"small":  true,
	"span":   true,
	"strong": true,
	"u":      true,
	"ul":     true,
}

var blockedSEOHTMLTags = map[string]bool{
	"iframe": true,
	"link":   true,
	"meta":   true,
	"object": true,
	"script": true,
	"style":  true,
}

var allowedSEOAttrs = map[string]map[string]bool{
	"a":   {"href": true, "rel": true, "target": true, "title": true},
	"img": {"alt": true, "height": true, "src": true, "title": true, "width": true},
}

var allowedSEOGlobalAttrs = map[string]bool{
	"aria-label": true,
}

func siteDisplayName(site settings.SiteSettings) string {
	name := strings.TrimSpace(site.Name)
	if name == "" {
		return "FluffCatch"
	}
	return name
}

func seoDocumentTitle(pageName string, siteName string) string {
	pageName = strings.TrimSpace(pageName)
	siteName = strings.TrimSpace(siteName)
	if siteName == "" {
		siteName = "FluffCatch"
	}
	if pageName == "" || pageName == "首页" {
		return siteName
	}
	return pageName + " - " + siteName
}

func (server *Server) seoPageName(r *stdhttp.Request) string {
	cleanPath := urlpath.Clean(r.URL.Path)
	switch {
	case cleanPath == "/" || cleanPath == ".":
		return "首页"
	case cleanPath == "/submit":
		return "返图入口"
	case cleanPath == "/login":
		return "登录"
	case cleanPath == "/admin" || cleanPath == "/admin/events" || cleanPath == "/admin/dashboard":
		return "活动管理"
	case cleanPath == "/admin/settings":
		return "系统设置"
	case strings.HasPrefix(cleanPath, "/admin/settings/"):
		return adminSettingsPageNameForSEO(strings.TrimPrefix(cleanPath, "/admin/settings/"))
	case strings.HasPrefix(cleanPath, "/events/"):
		if title := server.eventTitleForSEO(r, strings.TrimPrefix(cleanPath, "/events/")); title != "" {
			return title
		}
		return "活动详情"
	default:
		return ""
	}
}

func adminSettingsPageNameForSEO(section string) string {
	switch strings.Trim(strings.Split(section, "/")[0], " ") {
	case "site":
		return "站点信息"
	case "theme":
		return "主题配色"
	case "background":
		return "前台背景"
	case "contact":
		return "联系卡片"
	case "footer":
		return "页脚备案"
	case "upload":
		return "上传限制"
	case "storage":
		return "存储策略"
	case "maintenance":
		return "存储维护"
	case "security":
		return "账号安全"
	default:
		return "系统设置"
	}
}

func (server *Server) eventTitleForSEO(r *stdhttp.Request, rawID string) string {
	if server.db == nil {
		return ""
	}
	rawID = strings.Trim(strings.Split(rawID, "/")[0], " ")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return ""
	}
	var event appdb.Event
	if err := server.db.WithContext(r.Context()).Select("title").Where("id = ? AND is_public = ?", id, true).Take(&event).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(event.Title)
}

func absolutePublicURL(r *stdhttp.Request, value string) string {
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if !strings.HasPrefix(value, "/") {
		return value
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := firstForwardedValue(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = forwardedProto
	}
	host := r.Host
	if forwardedHost := firstForwardedValue(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		host = forwardedHost
	}
	if host == "" {
		return value
	}
	prefix := strings.TrimRight(strings.TrimSpace(firstForwardedValue(r.Header.Get("X-Forwarded-Prefix"))), "/")
	if prefix != "" && value != prefix && !strings.HasPrefix(value, prefix+"/") {
		value = prefix + value
	}
	return scheme + "://" + host + value
}

func parseIDParam(w stdhttp.ResponseWriter, r *stdhttp.Request, name string) (int64, bool) {
	raw := routeParam(r, name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, stdhttp.StatusBadRequest, fmt.Sprintf("invalid %s", name))
		return 0, false
	}

	return id, true
}

func parsePagination(r *stdhttp.Request, defaultPageSize int) (int, int) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if err != nil || pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize < 1 {
		pageSize = 24
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func parseOptionalDate(w stdhttp.ResponseWriter, value string, name string, endOfDay bool) (*time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, true
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, fmt.Sprintf("invalid %s", name))
		return nil, false
	}
	if endOfDay {
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
	}
	return &parsed, true
}

func parsePhotoVisibility(r *stdhttp.Request) (gallery.Visibility, bool) {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("visibility"))) {
	case "", "all":
		return "", true
	case "public":
		return gallery.VisibilityPublic, true
	case "private":
		return gallery.VisibilityPrivate, true
	default:
		return "", false
	}
}

func parseTagsValue(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})
	return fields
}

func sanitizeFilename(filename string) string {
	filename = filepath.Base(filename)
	filename = strings.ReplaceAll(filename, "\\", "_")
	filename = strings.ReplaceAll(filename, "/", "_")
	if strings.TrimSpace(filename) == "" {
		return "upload.bin"
	}
	return filename
}

func coverObjectKey(eventID int64, contentHash string, filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = ".bin"
	}
	return fmt.Sprintf("events/%d/cover/%s%s", eventID, contentHash, ext)
}

func logoObjectKey(contentHash string, filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = ".bin"
	}
	return fmt.Sprintf("site/logo/%s%s", contentHash, ext)
}

func siteBackgroundObjectKey(variant string, contentHash string) string {
	return fmt.Sprintf("site/backgrounds/%s/%s.jpg", variant, contentHash)
}

func (server *Server) viewerFingerprintHash(w stdhttp.ResponseWriter, r *stdhttp.Request) string {
	viewerID := server.viewerID(w, r)
	sum := sha256.Sum256([]byte(viewerID))
	return hex.EncodeToString(sum[:])
}

func (server *Server) viewerID(w stdhttp.ResponseWriter, r *stdhttp.Request) string {
	if cookie, err := r.Cookie(viewerCookieName); err == nil {
		value := strings.TrimSpace(cookie.Value)
		if isValidViewerID(value) {
			return value
		}
	}

	value := newViewerID()
	secure := r.TLS != nil || server.cfg.App.Env == "production"
	stdhttp.SetCookie(w, &stdhttp.Cookie{
		Name:     viewerCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   viewerCookieMaxAge,
		HttpOnly: true,
		SameSite: stdhttp.SameSiteLaxMode,
		Secure:   secure,
	})
	return value
}

func newViewerID() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		fallback := sha256.Sum256([]byte(strings.Join([]string{time.Now().String(), strconv.FormatInt(time.Now().UnixNano(), 10)}, "|")))
		return base64.RawURLEncoding.EncodeToString(fallback[:])
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func isValidViewerID(value string) bool {
	if len(value) < 32 || len(value) > 96 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func clientIPFromRequest(r *stdhttp.Request) string {
	ip := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if first := strings.TrimSpace(parts[0]); first != "" {
			return first
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	return ip
}
