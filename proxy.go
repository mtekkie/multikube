package multikube

import (
	"log"
	"net"
	"fmt"
	"context"
	"net/http"
	"crypto/tls"
	"net/http/httputil"
	"k8s.io/client-go/tools/clientcmd/api"
)

type Proxy struct {
	Config *Config
	config *api.Config
	mw     http.Handler
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// NewProxy crerates a new Proxy and initialises router and configuration
func NewProxy() *Proxy {
	return &Proxy{
		Config: &Config{},
	}
}

func NewProxyFrom(c *api.Config) *Proxy {

	p := NewProxy()
	p.config = c
	return p
	
}

// Use chains all middlewares and applies a context to the request flow
func (p *Proxy) use(mw ...MiddlewareFunc) MiddlewareFunc {
	return func(final http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			last := final
			for i := len(mw) - 1; i >= 0; i-- {
				last = mw[i](last)
			}
			ctx := context.WithValue(r.Context(), "config", p.Config)
			last(w, r.WithContext(ctx))
		}
	}
}

// Use chains all middlewares and applies a context to the request flow
func (p *Proxy) Use(mw ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			last := final
			for i := len(mw) - 1; i >= 0; i-- {
				last = mw[i](last)
			}
			last.ServeHTTP(w, r)
		})
	}
}

func (p *Proxy) getCluster(n string) *api.Cluster {
	for k, v := range p.config.Clusters {
		if k == n {
			return v
		}
	}
	return nil
}

func (p *Proxy) getAuthInfo(n string) *api.AuthInfo {
	for k, v := range p.config.AuthInfos {
		if k == n {
			return v
		}
	}
	return nil
}

func (p *Proxy) getContext(n string) *api.Context {
	for k, v := range p.config.Contexts {
		if k == n {
			return v
		}
	}
	return nil
}

func (p *Proxy) getOptions(n string) *Options {
	ctx := p.getContext(n)
	if ctx == nil {
		return nil
	}
	authInfo := p.getAuthInfo(ctx.AuthInfo)
	if authInfo == nil {
		return nil
	}
	cluster := p.getCluster(ctx.Cluster)
	if cluster == nil {
		return nil
	}
	return &Options{
		cluster,
		authInfo,
	}
}

// ServeHTTP routes the request to an apiserver. It determines, resolves an apiserver using
// data in the request itsel such as certificate data, authorization bearer tokens, http headers etc.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	target := r.Context().Value("Context").(string)
	opts := p.getOptions(target)
	
	if opts == nil {
		http.Error(w, fmt.Sprintf("Unable to resolve context %s", target), http.StatusInternalServerError)
		return
	}

	// Tunnel the connection if server sends Upgrade
	if r.Header.Get("Upgrade") != "" {
		p.tunnel(w, r)
		return
	}

	// Build the request and execute the call to the backend apiserver
	req :=
		NewRequest(opts).
			Method(r.Method).
			Body(r.Body).
			Path(r.URL.Path).
			Query(r.URL.RawQuery).
			Headers(r.Header)

	// Execute!
	res, err := req.Do()
	defer res.Body.Close()

	// Catch any unexpected errors
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy all response headers
	copyHeader(w.Header(), res.Header)
	w.WriteHeader(res.StatusCode)

	// Read body into buffer before writing to response and wait until client cancels
	buf := make([]byte, 4096)
	for {
		n, err := res.Body.Read(buf)
		if n == 0 && err != nil {
			break
		}
		b := buf[:n]
		w.Write(b)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

}

// tunnel hijacks the client request, creates a pipe between client and backend server
// and starts streaming data between the two connections.
func (p *Proxy) tunnel(w http.ResponseWriter, r *http.Request) {

	target := r.Context().Value("Context").(string)
	opts := p.getOptions(target)
	
	if opts == nil {
		log.Printf("Unable to resolve target '%s'", target)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	req := NewRequest(opts)

	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	dst_conn, err := tls.Dial("tcp", "192.168.99.100:8443", req.TLSConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	dst_conn.Write(dump)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	src_conn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	go transfer(dst_conn, src_conn)
	go transfer(src_conn, dst_conn)

}

// transfer reads the data from src into a buffer before it writes it into dst
func transfer(src, dst net.Conn) {
	buff := make([]byte, 65535)
	defer src.Close()
	defer dst.Close()

	for {
		n, err := src.Read(buff)
		if err != nil {
			break
		}
		b := buff[:n]
		_, err = dst.Write(b)
		if err != nil {
			break
		}
	}

	log.Printf("Transfered src: %s dst: %s bytes: %d", src.LocalAddr().String(), dst.RemoteAddr().String(), len(buff))
}
