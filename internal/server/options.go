package server

import "time"

type Options struct {
	Port           int
	Auth           Optional[AuthOptions]
	Tls            Optional[TlsOptions]
	UpdateInterval time.Duration
}

type Optional[v any] struct {
	Enabled bool
	value   v
}

func (o *Optional[v]) Get() v {
	return o.value
}

func (o *Optional[v]) Set(value v) {
	o.value = value
	o.Enabled = true
}

func (o *Optional[v]) Apply(apply func(*v)) {
	o.Enabled = true
	apply(&o.value)
}

type AuthType int

const (
	Raw AuthType = iota
	File
)

type AuthOptions struct {
	// Secret Direct or Path to secret File
	Secret   string
	LoadType AuthType
	Secure   bool
	MaxAge   int
}

type TlsOptions struct {
	CertPath string
	KeyPath  string
}

func NewDefaultOptions() Options {
	return Options{
		Port: 8080,
		Auth: Optional[AuthOptions]{
			Enabled: false,
		},
		Tls: Optional[TlsOptions]{
			Enabled: false,
		},
		UpdateInterval: 15 * time.Minute,
	}
}
