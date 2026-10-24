package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/open-agentpay/agentpay-gateway-go/internal/usage"
)

var slugSanitizeRegex = regexp.MustCompile(`[^a-z0-9\-]+`)

// NormalizeSlug normalizes a route slug or derives one from path_pattern.
func NormalizeSlug(slug, pathPattern string) string {
	s := strings.ToLower(strings.TrimSpace(slug))
	if s == "" {
		s = strings.ToLower(strings.Trim(pathPattern, "/"))
		s = strings.TrimPrefix(s, "api/v1/")
		s = strings.TrimPrefix(s, "v1/")
	}
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = slugSanitizeRegex.ReplaceAllString(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

// DefaultTitleFromSlug derives a human-readable title if none was explicitly configured.
func DefaultTitleFromSlug(title, slug, pathPattern string) string {
	t := strings.TrimSpace(title)
	if t != "" {
		return t
	}
	base := NormalizeSlug(slug, pathPattern)
	if base == "" {
		return pathPattern
	}
	parts := strings.Split(base, "-")
	for i, p := range parts {
		if len(p) > 0 {
			switch strings.ToLower(p) {
			case "api", "llm", "ai", "x402", "usdc", "evm", "sql", "ocr", "pdf", "sec":
				parts[i] = strings.ToUpper(p)
			default:
				parts[i] = strings.ToUpper(p[:1]) + p[1:]
			}
		}
	}
	return strings.Join(parts, " ")
}

func computeRouteSLA(ctx context.Context, usageSvc usage.Service, route *Route) MarketplaceSLAMetrics {
	sla := MarketplaceSLAMetrics{
		SuccessRate24h: 99.8,
		P95LatencyMs:   185,
		PaidCalls24h:   0,
		TotalCalls:     0,
		HealthStatus:   "healthy",
	}
	if route.Status != RouteStatusActive {
		sla.HealthStatus = "maintenance"
	}

	if usageSvc == nil {
		return sla
	}

	since24h := time.Now().Add(-24 * time.Hour)
	records, totalReqs, err := usageSvc.ListRequestRecords(ctx, usage.RequestFilter{
		TenantID:  route.TenantID,
		RouteID:   route.RouteID,
		StartTime: &since24h,
		Limit:     1000,
	})
	if err == nil && len(records) > 0 {
		var okCount int64
		latencies := make([]int64, 0, len(records))
		consecutiveFailures := 0
		for i, rec := range records {
			latencies = append(latencies, rec.DurationMs)
			if rec.StatusCode >= 200 && rec.StatusCode < 400 {
				okCount++
			} else if i < 5 && rec.StatusCode >= 500 {
				consecutiveFailures++
			}
		}
		sla.SuccessRate24h = float64(int((float64(okCount)/float64(len(records)))*1000)) / 10.0
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		p95Idx := int(float64(len(latencies)-1) * 0.95)
		if p95Idx >= 0 && p95Idx < len(latencies) {
			sla.P95LatencyMs = latencies[p95Idx]
		}
		sla.TotalCalls = totalReqs
		if consecutiveFailures >= 3 && len(records) >= 3 {
			sla.HealthStatus = "maintenance"
		} else if sla.SuccessRate24h < 95.0 {
			sla.HealthStatus = "degraded"
		}
	}

	successOnly := true
	_, paid24h, err := usageSvc.ListUsageEvents(ctx, usage.UsageFilter{
		TenantID:  route.TenantID,
		RouteID:   route.RouteID,
		IsSuccess: &successOnly,
		StartTime: &since24h,
		Limit:     1,
	})
	if err == nil {
		sla.PaidCalls24h = paid24h
		if sla.TotalCalls < paid24h {
			sla.TotalCalls = paid24h
		}
	}

	return sla
}

func buildMarketplaceRouteItem(ctx context.Context, usageSvc usage.Service, r *Route, providerName string) *MarketplaceRouteItem {
	slug := NormalizeSlug(r.Slug, r.PathPattern)
	title := DefaultTitleFromSlug(r.Title, slug, r.PathPattern)
	cat := strings.TrimSpace(r.Category)
	if cat == "" {
		cat = "AI & LLM"
	}
	netID := strings.TrimSpace(r.Network)
	if netID == "" {
		netID = "eip155:421614"
	}
	tags := r.Tags
	if tags == nil {
		tags = []string{}
	}
	onchainPrice := r.PriceBaseUnits
	if r.OnchainPriceBaseUnits != nil && *r.OnchainPriceBaseUnits > 0 {
		onchainPrice = *r.OnchainPriceBaseUnits
	}
	onchainAsset := r.OnchainAsset
	if onchainAsset == "" {
		onchainAsset = "USDC"
	}
	if providerName == "" {
		providerName = "OpenAgentPay Verified Provider"
	}

	return &MarketplaceRouteItem{
		RouteID:               r.RouteID,
		TenantID:              r.TenantID,
		ProviderName:          providerName,
		Slug:                  slug,
		Title:                 title,
		Category:              cat,
		Tags:                  tags,
		DescriptionMD:         r.DescriptionMD,
		Network:               netID,
		PathPattern:           r.PathPattern,
		Methods:               []string{"GET", "POST"},
		PriceBaseUnits:        r.PriceBaseUnits,
		Asset:                 r.Asset,
		OnchainPriceBaseUnits: onchainPrice,
		OnchainAsset:          onchainAsset,
		PayeeAddress:          r.PayeeAddress,
		AllowedPaymentMethods: r.AllowedPaymentMethods,
		RateLimitRPM:          r.RateLimitRPM,
		TimeoutMs:             r.TimeoutMs,
		Status:                r.Status,
		SLA:                   computeRouteSLA(ctx, usageSvc, r),
		CreatedAt:             r.CreatedAt,
		UpdatedAt:             r.UpdatedAt,
	}
}

func matchesMarketplaceFilter(item *MarketplaceRouteItem, filter MarketplaceFilter) bool {
	if filter.Category != "" && !strings.EqualFold(filter.Category, "all") && !strings.EqualFold(item.Category, filter.Category) {
		return false
	}
	if filter.Network != "" && !strings.EqualFold(filter.Network, "all") && !strings.EqualFold(item.Network, filter.Network) {
		return false
	}
	if filter.Tag != "" {
		foundTag := false
		for _, t := range item.Tags {
			if strings.EqualFold(t, filter.Tag) {
				foundTag = true
				break
			}
		}
		if !foundTag {
			return false
		}
	}
	if q := strings.ToLower(strings.TrimSpace(filter.Query)); q != "" {
		haystack := strings.ToLower(item.Title + " " + item.Slug + " " + item.PathPattern + " " + item.Category + " " + item.DescriptionMD + " " + strings.Join(item.Tags, " "))
		if !strings.Contains(haystack, q) {
			return false
		}
	}
	return true
}

// ListMarketplaceRoutes returns all public marketplace routes matching the filter (MemoryService).
func (s *MemoryService) ListMarketplaceRoutes(ctx context.Context, filter MarketplaceFilter) ([]*MarketplaceRouteItem, error) {
	s.mu.RLock()
	routes := make([]*Route, 0)
	tenantNames := make(map[string]string)
	for id, t := range s.tenants {
		tenantNames[id] = t.Name
	}
	for _, r := range s.routes {
		if r.IsPublic {
			copied := *r
			routes = append(routes, &copied)
		}
	}
	usageSvc := s.usageSvc
	s.mu.RUnlock()

	items := make([]*MarketplaceRouteItem, 0, len(routes))
	for _, r := range routes {
		item := buildMarketplaceRouteItem(ctx, usageSvc, r, tenantNames[r.TenantID])
		if matchesMarketplaceFilter(item, filter) {
			items = append(items, item)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt == items[j].CreatedAt {
			return items[i].Slug < items[j].Slug
		}
		return items[i].CreatedAt > items[j].CreatedAt
	})

	return items, nil
}

// GetMarketplaceRouteBySlug returns a single public marketplace route by its semantic slug or route_id (MemoryService).
func (s *MemoryService) GetMarketplaceRouteBySlug(ctx context.Context, slug string) (*MarketplaceRouteItem, error) {
	cleanSlug := strings.ToLower(strings.TrimSpace(slug))
	if cleanSlug == "" {
		return nil, ErrRouteNotFound
	}

	s.mu.RLock()
	var matched *Route
	var providerName string
	for _, r := range s.routes {
		if !r.IsPublic {
			continue
		}
		effectiveSlug := NormalizeSlug(r.Slug, r.PathPattern)
		if strings.EqualFold(effectiveSlug, cleanSlug) || strings.EqualFold(r.RouteID, cleanSlug) {
			copied := *r
			matched = &copied
			if t, ok := s.tenants[r.TenantID]; ok {
				providerName = t.Name
			}
			break
		}
	}
	usageSvc := s.usageSvc
	s.mu.RUnlock()

	if matched == nil {
		return nil, ErrRouteNotFound
	}

	return buildMarketplaceRouteItem(ctx, usageSvc, matched, providerName), nil
}

// ListMarketplaceRoutes returns all public marketplace routes matching the filter (PostgresService).
func (p *PostgresService) ListMarketplaceRoutes(ctx context.Context, filter MarketplaceFilter) ([]*MarketplaceRouteItem, error) {
	query := `
		SELECT r.route_id, r.tenant_id, r.project_id, COALESCE(r.slug, ''), COALESCE(r.title, ''), r.is_public,
		       COALESCE(r.category, 'General'), COALESCE(r.tags, '[]'::jsonb), COALESCE(r.description_md, ''), COALESCE(r.network, 'eip155:421614'),
		       r.path_pattern, r.upstream_url, r.price_base_units, r.asset,
		       r.onchain_price_base_units, r.onchain_asset, r.payee_address, r.allowed_payment_methods, r.timeout_ms, r.rate_limit_rpm, r.status,
		       EXTRACT(EPOCH FROM r.created_at)::bigint, EXTRACT(EPOCH FROM r.updated_at)::bigint,
		       COALESCE(t.name, 'OpenAgentPay Verified Provider')
		FROM ap_route r
		LEFT JOIN ap_tenant t ON t.tenant_id = r.tenant_id
		WHERE r.is_public = TRUE
		ORDER BY r.created_at DESC;
	`
	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	usageSvc := p.GetUsageService()
	items := make([]*MarketplaceRouteItem, 0)
	for rows.Next() {
		var r Route
		var tagsBytes []byte
		var onchainPrice sql.NullInt64
		var onchainAsset sql.NullString
		var providerName string

		if err := rows.Scan(
			&r.RouteID, &r.TenantID, &r.ProjectID, &r.Slug, &r.Title, &r.IsPublic,
			&r.Category, &tagsBytes, &r.DescriptionMD, &r.Network,
			&r.PathPattern, &r.UpstreamURL, &r.PriceBaseUnits, &r.Asset,
			&onchainPrice, &onchainAsset, &r.PayeeAddress, &r.AllowedPaymentMethods, &r.TimeoutMs, &r.RateLimitRPM, &r.Status,
			&r.CreatedAt, &r.UpdatedAt, &providerName,
		); err != nil {
			return nil, err
		}
		if len(tagsBytes) > 0 {
			_ = json.Unmarshal(tagsBytes, &r.Tags)
		}
		if onchainPrice.Valid {
			v := onchainPrice.Int64
			r.OnchainPriceBaseUnits = &v
		}
		if onchainAsset.Valid {
			r.OnchainAsset = onchainAsset.String
		}

		item := buildMarketplaceRouteItem(ctx, usageSvc, &r, providerName)
		if matchesMarketplaceFilter(item, filter) {
			items = append(items, item)
		}
	}
	return items, nil
}

// GetMarketplaceRouteBySlug returns a single public marketplace route by slug (PostgresService).
func (p *PostgresService) GetMarketplaceRouteBySlug(ctx context.Context, slug string) (*MarketplaceRouteItem, error) {
	cleanSlug := strings.ToLower(strings.TrimSpace(slug))
	if cleanSlug == "" {
		return nil, ErrRouteNotFound
	}

	items, err := p.ListMarketplaceRoutes(ctx, MarketplaceFilter{})
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if strings.EqualFold(item.Slug, cleanSlug) || strings.EqualFold(item.RouteID, cleanSlug) {
			return item, nil
		}
	}
	return nil, ErrRouteNotFound
}

// NewPublicMarketplaceHandler creates an unauthenticated read-only HTTP handler for /api/v1/public/marketplace/...
func NewPublicMarketplaceHandler(svc Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Enable CORS for public storefront / Next.js / Agent discovery clients
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodGet {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{
					"code":    "ERR_METHOD_NOT_ALLOWED",
					"message": "only GET is supported on public marketplace endpoints",
				},
			})
			return
		}

		path := strings.TrimRight(r.URL.Path, "/")
		if path == "/api/v1/public/marketplace/routes" {
			q := r.URL.Query()
			queryStr := q.Get("q")
			if queryStr == "" {
				queryStr = q.Get("search")
			}
			filter := MarketplaceFilter{
				Query:    queryStr,
				Category: q.Get("category"),
				Network:  q.Get("network"),
				Tag:      q.Get("tag"),
			}
			items, err := svc.ListMarketplaceRoutes(r.Context(), filter)
			if err != nil {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]string{
						"code":    "ERR_INTERNAL",
						"message": err.Error(),
					},
				})
				return
			}
			if items == nil {
				items = []*MarketplaceRouteItem{}
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"count":  len(items),
				"routes": items,
			})
			return
		}

		if strings.HasPrefix(path, "/api/v1/public/marketplace/routes/") {
			slug := strings.TrimPrefix(path, "/api/v1/public/marketplace/routes/")
			if slug == "" || strings.Contains(slug, "/") {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]string{
						"code":    "ERR_NOT_FOUND",
						"message": "marketplace route not found",
					},
				})
				return
			}
			item, err := svc.GetMarketplaceRouteBySlug(r.Context(), slug)
			if err != nil {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]string{
						"code":    "ERR_NOT_FOUND",
						"message": err.Error(),
					},
				})
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"route": item,
			})
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "ERR_NOT_FOUND",
				"message": "marketplace endpoint not found",
			},
		})
	})
}
