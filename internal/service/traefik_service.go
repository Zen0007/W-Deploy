package service

import (
	"fmt"
	"strings"

	"W-dev/internal/constants"
)

// TraefikService membangun label Docker agar Traefik me-route hostname ke container app.
// Traefik membaca label secara dinamis (provider docker), jadi tidak ada file config per app.
type TraefikService struct {
	network      string
	tls          bool
	certResolver string
	mainDomain   string
}

func NewTraefikService(network, mainDomain string, tls bool, certResolver string) *TraefikService {
	return &TraefikService{network: network, mainDomain: mainDomain, tls: tls, certResolver: certResolver}
}

// Hostname mengembalikan FQDN publik app: <slug>.<MAIN_DOMAIN>.
func (s *TraefikService) Hostname(slug string) string {
	return fmt.Sprintf("%s.%s", slug, s.mainDomain)
}

// PublicURL URL publik app.
func (s *TraefikService) PublicURL(slug string) string {
	// Dengan Cloudflare Tunnel, pengunjung selalu mengakses via https di edge Cloudflare.
	scheme := "https"
	if strings.HasPrefix(s.mainDomain, "localhost") {
		scheme = "http"
	}
	return scheme + "://" + s.Hostname(slug)
}

// Labels membangun label container untuk sebuah app.
func (s *TraefikService) Labels(appID uint, slug string, port int) map[string]string {
	router := "paas-" + slug
	host := fmt.Sprintf("Host(`%s`)", s.Hostname(slug))

	labels := map[string]string{
		constants.LabelManaged: "true",
		constants.LabelAppID:   fmt.Sprintf("%d", appID),
		"paas.app.slug":        slug,

		"traefik.enable":         "true",
		"traefik.docker.network": s.network,

		fmt.Sprintf("traefik.http.routers.%s.rule", router):                      host,
		fmt.Sprintf("traefik.http.routers.%s.service", router):                   router,
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", router): fmt.Sprintf("%d", port),
	}

	if s.tls {
		labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", router)] = "websecure"
		labels[fmt.Sprintf("traefik.http.routers.%s.tls", router)] = "true"
		labels[fmt.Sprintf("traefik.http.routers.%s.tls.certresolver", router)] = s.certResolver

		// Redirect http -> https
		labels[fmt.Sprintf("traefik.http.routers.%s-http.rule", router)] = host
		labels[fmt.Sprintf("traefik.http.routers.%s-http.entrypoints", router)] = "web"
		labels[fmt.Sprintf("traefik.http.routers.%s-http.middlewares", router)] = router + "-redirect"
		labels[fmt.Sprintf("traefik.http.middlewares.%s-redirect.redirectscheme.scheme", router)] = "https"
		labels[fmt.Sprintf("traefik.http.middlewares.%s-redirect.redirectscheme.permanent", router)] = "true"
	} else {
		// TLS diterminasi di Cloudflare; tunnel meneruskan HTTP ke entrypoint "web".
		labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", router)] = "web"
	}
	return labels
}
